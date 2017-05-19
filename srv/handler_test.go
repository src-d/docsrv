package srv

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRedirectToLatest(t *testing.T) {
	github := newGitHubMock()
	srv := newTestSrv(github)

	github.add("proj1", "v1.0.0", "foo")
	github.add("proj1", "v0.9.0", "foo")

	assertRedirect(
		t, srv,
		"http://proj1.foo.bar/latest/",
		"http://proj1.foo.bar/v1.0.0",
	)

	// add a new version and receive the previous one because
	// it is cached
	github.add("proj1", "v2.0.0", "baz")

	assertRedirect(
		t, srv,
		"http://proj1.foo.bar/latest/",
		"http://proj1.foo.bar/v1.0.0",
	)

	// if the path contains something, redirect will have it as well
	assertRedirect(
		t, srv,
		"http://proj1.foo.bar/latest/foo",
		"http://proj1.foo.bar/v1.0.0/foo",
	)

	// no versions available
	assertNotFound(t, srv, "http://proj2.foo.bar/latest/")
}

func TestProjectNameFromReq(t *testing.T) {
	cases := []struct {
		url      string
		expected string
	}{
		{"http://foo.bar.baz.bax/fooo", "foo"},
		{"http://foo.bax/fooo", "foo"},
		{"http://localhost/fooo", "localhost"},
	}

	for _, c := range cases {
		req, err := http.NewRequest("GET", c.url, nil)
		require.NoError(t, err, c.url)

		require.Equal(t, c.expected, projectNameFromReq(req), c.url)
	}
}

func TestVersionNameFromReq(t *testing.T) {
	cases := []struct {
		url      string
		expected string
	}{
		{"http://foo/fooo", "fooo"},
		{"http://foo/fooo/bar", "fooo"},
	}

	for _, c := range cases {
		req, err := http.NewRequest("GET", c.url, nil)
		require.NoError(t, err, c.url)

		require.Equal(t, c.expected, versionFromReq(req), c.url)
	}
}

const expectedDocsOutput = `%s
/etc/shared
`

func TestBuildDocs(t *testing.T) {
	require := require.New(t)
	url, close := tarGzServer()
	defer close()

	tmpDir, err := ioutil.TempDir("", "docsrv-test-")
	require.NoError(err)

	require.NoError(buildDocs(url, "http://foo.bar", tmpDir))
	assertMakefileOutput(t, tmpDir, "http://foo.bar")
}

func TestPrepareVersion(t *testing.T) {
	require := require.New(t)
	url, close := tarGzServer()
	defer close()

	tmpDir, err := ioutil.TempDir("", "docsrv-test-")
	require.NoError(err)

	github := newGitHubMock()
	srv := newTestSrv(github)
	srv.baseFolder = tmpDir

	github.add("foo", "v1.0.0", url)

	assertRedirect(
		t, srv,
		"http://foo.bar.baz/v1.0.0/something",
		"http://foo.bar.baz/v1.0.0/something",
	)

	assertMakefileOutput(t,
		filepath.Join(tmpDir, "foo.bar.baz", "v1.0.0"),
		"http://foo.bar.baz/v1.0.0",
	)
}

func assertMakefileOutput(t *testing.T, tmpDir, baseURL string) {
	fp := filepath.Join(tmpDir, "out")
	data, err := ioutil.ReadFile(fp)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf(expectedDocsOutput, baseURL), string(data))
}

func assertNotFound(t *testing.T, handler http.Handler, requestURL string) {
	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", requestURL, nil)
	require.NoError(t, err, "unexpected error creating request")

	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "expected a not found")
}

func assertInternalError(t *testing.T, handler http.Handler, requestURL string) {
	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", requestURL, nil)
	require.NoError(t, err, "unexpected error creating request")

	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code, "expected a not found")
}

func assertRedirect(t *testing.T, handler http.Handler, requestURL, expected string) {
	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", requestURL, nil)
	require.NoError(t, err, "unexpected error creating request")

	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusTemporaryRedirect, w.Code, "expected a redirect")
	url := w.Header().Get("Location")
	require.Equal(t, expected, url, "wrong redirect url")
}

type gitHubMock struct {
	releases map[string]map[string]string
}

func newGitHubMock() *gitHubMock {
	return &gitHubMock{make(map[string]map[string]string)}
}

func (m *gitHubMock) add(project, version, url string) {
	if _, ok := m.releases[project]; !ok {
		m.releases[project] = make(map[string]string)
	}

	m.releases[project][version] = url
}

func (m *gitHubMock) Releases(project string) ([]*Release, error) {
	if proj, ok := m.releases[project]; ok {
		var releases []*Release
		for v, url := range proj {
			releases = append(releases, &Release{
				Tag:  v,
				Docs: url,
			})
		}
		sort.Sort(byTag(releases))
		return releases, nil
	}

	return nil, nil
}

func (m *gitHubMock) Release(project, version string) (*Release, error) {
	if proj, ok := m.releases[project]; ok {
		if rel, ok := proj[version]; ok {
			return &Release{
				Tag:  version,
				Docs: rel,
			}, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func newTestSrv(github GitHub) *DocSrv {
	return &DocSrv{
		"",
		github,
		new(sync.RWMutex),
		make(map[string]latestVersion),
		new(sync.RWMutex),
		make(map[string]struct{}),
	}
}

func tarGzServer() (string, func()) {
	server := httptest.NewServer(http.HandlerFunc(tarGzMakefileHandler))
	return server.URL, server.Close
}

const testMakefile = `
build:
	@OUTPUT=$(DESTINATION_FOLDER)/out; \
	echo "$(BASE_URL)" >> $$OUTPUT; \
	echo "$(SHARED_REPO_FOLDER)" >> $$OUTPUT;
`

func tarGzMakefileHandler(w http.ResponseWriter, r *http.Request) {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err := tw.WriteHeader(&tar.Header{
		Name:    "Makefile",
		Mode:    0777,
		Size:    int64(len([]byte(testMakefile))),
		ModTime: time.Now(),
	})
	if err != nil {
		return
	}

	io.Copy(tw, bytes.NewBuffer([]byte(testMakefile)))
}
