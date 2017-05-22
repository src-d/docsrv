package srv

import (
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

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
	assertRedirect(
		t, srv,
		"http://proj2.foo.bar/latest/",
		"/404.html",
	)
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

	require.Len(srv.versions["foo"], 0)

	assertRedirect(
		t, srv,
		"http://foo.bar.baz/v1.0.0/something",
		"http://foo.bar.baz/v1.0.0/something",
	)

	assertMakefileOutput(t,
		filepath.Join(tmpDir, "foo.bar.baz", "v1.0.0"),
		"http://foo.bar.baz/v1.0.0",
	)

	require.Len(srv.versions["foo"], 1)
}

func TestListVersions(t *testing.T) {
	github := newGitHubMock()
	srv := newTestSrv(github)
	github.add("foo", "v1.0.0", "")
	github.add("foo", "v1.1.0", "")
	github.add("foo", "v1.2.0", "")
	github.add("bar", "v1.3.0", "")

	assertJSON(t, srv, "http://foo.bar.baz/versions.json", []*version{
		{"v1.0.0", "http://foo.bar.baz/v1.0.0"},
		{"v1.1.0", "http://foo.bar.baz/v1.1.0"},
		{"v1.2.0", "http://foo.bar.baz/v1.2.0"},
	})
}
