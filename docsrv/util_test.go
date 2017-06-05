package docsrv

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/require"
)

func assertJSON(t *testing.T, handler http.Handler, requestURL string, expected interface{}) {
	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", requestURL, nil)
	require.NoError(t, err, "unexpected error creating request")

	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err, "unexpected error marshaling json")
	require.Equal(t, string(expectedJSON), w.Body.String())
}

const expectedDocsOutput = `%s
%s
%s
%s
/etc/shared
true
`

func assertMakefileOutput(t *testing.T, tmpDir, baseURL, project, owner, version string) {
	fp := filepath.Join(tmpDir, "out")
	data, err := ioutil.ReadFile(fp)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf(expectedDocsOutput, baseURL, project, owner, version), string(data))
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

type mockFetcher struct {
	projectReleases map[string]map[string]string
}

func newMockFetcher() *mockFetcher {
	return &mockFetcher{
		make(map[string]map[string]string),
	}
}

func (m *mockFetcher) add(owner, project, version, url string) {
	key := filepath.Join(owner, project)
	if _, ok := m.projectReleases[key]; !ok {
		m.projectReleases[key] = make(map[string]string)
	}

	m.projectReleases[key][version] = url
}

func (m *mockFetcher) releases(owner, project string, minVersion *semver.Version) ([]*release, error) {
	key := filepath.Join(owner, project)
	if proj, ok := m.projectReleases[key]; ok {
		var releases []*release
		for v, url := range proj {
			release := &release{
				tag: v,
				url: url,
			}

			v := newVersion(release.tag)
			if v != nil && v.LessThan(minVersion) {
				continue
			}
			releases = append(releases, release)
		}
		sort.Sort(byTag(releases))
		return releases, nil
	}

	return nil, nil
}

func newTestSrv(fetcher releaseFetcher, config Config) *Service {
	srv := New(Options{Config: config})
	srv.fetcher = fetcher
	return srv
}

func tarGzServer() (string, func()) {
	server := httptest.NewServer(http.HandlerFunc(tarGzMakefileHandler))
	return server.URL, server.Close
}

const testMakefile = `
docs:
	@OUTPUT=$(DESTINATION_FOLDER)/out; \
	echo "$(BASE_URL)" >> $$OUTPUT; \
	echo "$(REPOSITORY)" >> $$OUTPUT; \
	echo "$(REPOSITORY_OWNER)" >> $$OUTPUT; \
	echo "$(VERSION_NAME)" >> $$OUTPUT; \
	echo "$(SHARED_FOLDER)" >> $$OUTPUT; \
	echo "$(DOCSRV)" >> $$OUTPUT;
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
