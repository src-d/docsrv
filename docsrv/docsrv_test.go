package docsrv

import (
	"context"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRedirectToLatest(t *testing.T) {
	fetcher := newMockFetcher()
	srv := newTestSrv(fetcher)
	srv.opts.DefaultOwner = "org"

	fetcher.add("org", "proj1", "v1.0.0", "foo")
	fetcher.add("org", "proj1", "v0.9.0", "foo")

	assertRedirect(
		t, srv,
		"http://proj1.foo.bar/latest/",
		"http://proj1.foo.bar/v1.0.0/",
	)

	// add a new version and receive the previous one because
	// it is cached
	fetcher.add("org", "proj1", "v2.0.0", "baz")

	assertRedirect(
		t, srv,
		"http://proj1.foo.bar/latest/",
		"http://proj1.foo.bar/v1.0.0/",
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

func TestRedirectToLatest_WithMapping(t *testing.T) {
	fetcher := newMockFetcher()
	srv := newTestSrv(fetcher)
	srv.opts.DefaultOwner = "org"
	srv.opts.Mappings = Mappings{
		"proj1.foo.bar": "org2/proj1",
	}

	fetcher.add("org", "proj1", "v1.0.0", "foo")
	fetcher.add("org2", "proj1", "v0.9.0", "foo")

	assertRedirect(
		t, srv,
		"http://proj1.foo.bar/latest/",
		"http://proj1.foo.bar/v0.9.0/",
	)
}

func TestRedirectToLatest_RefreshToken(t *testing.T) {
	fetcher := newMockFetcher()
	srv := newTestSrv(fetcher)
	srv.opts.RefreshToken = "foo"
	srv.opts.DefaultOwner = "org"
	fetcher.add("org", "proj1", "v1.0.0", "foo")
	require.NoError(t, srv.indexProject("org", "proj1"))
	fetcher.add("org", "proj1", "v1.1.0", "foo")

	assertRedirect(
		t, srv,
		"http://proj1.foo.bar/latest/?token=foo",
		"http://proj1.foo.bar/v1.1.0/",
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

	fetcher := newMockFetcher()
	srv := newTestSrv(fetcher)
	srv.opts.DefaultOwner = "bar"
	srv.opts.BaseFolder = tmpDir
	srv.opts.SharedFolder = "/etc/shared"

	fetcher.add("bar", "foo", "v1.0.0", url)

	require.Len(srv.index.projects["bar/foo"], 0)

	assertRedirect(
		t, srv,
		"http://foo.bar.baz/v1.0.0/something",
		"http://foo.bar.baz/v1.0.0/something",
	)

	assertMakefileOutput(t,
		filepath.Join(tmpDir, "foo.bar.baz", "v1.0.0"),
		"http://foo.bar.baz/v1.0.0",
		"foo",
		"bar",
		"v1.0.0",
	)

	require.Len(srv.index.projects["bar/foo"], 1)
}

func TestPrepareVersion_RefreshToken(t *testing.T) {
	require := require.New(t)
	url, close := tarGzServer()
	defer close()

	tmpDir, err := ioutil.TempDir("", "docsrv-test-")
	require.NoError(err)

	fetcher := newMockFetcher()
	srv := newTestSrv(fetcher)
	srv.opts.DefaultOwner = "bar"
	srv.opts.BaseFolder = tmpDir
	srv.opts.SharedFolder = "/etc/shared"
	srv.opts.RefreshToken = "refresh"

	fetcher.add("bar", "foo", "v1.0.0", url)

	require.Len(srv.index.projects["bar/foo"], 0)

	assertRedirect(
		t, srv,
		"http://foo.bar.baz/v1.0.0/something",
		"http://foo.bar.baz/v1.0.0/something",
	)

	fetcher.add("bar", "foo", "v1.1.0", url)

	// without refresh token it's not updated
	assertRedirect(t, srv, "http://foo.bar.baz/v1.1.0/", "/404.html")

	// with refresh token it's updated
	assertRedirect(
		t, srv,
		"http://foo.bar.baz/v1.1.0/?token=refresh",
		"http://foo.bar.baz/v1.1.0/?token=refresh",
	)
}

func TestListVersions(t *testing.T) {
	fetcher := newMockFetcher()
	srv := newTestSrv(fetcher)
	srv.opts.DefaultOwner = "org"
	fetcher.add("org", "foo", "v1.0.0", "")
	fetcher.add("org", "foo", "v1.1.0", "")
	fetcher.add("org", "foo", "v1.2.0", "")
	fetcher.add("org", "bar", "v1.3.0", "")

	assertJSON(t, srv, "http://foo.bar.baz/versions.json", []*version{
		{"v1.0.0", "http://foo.bar.baz/v1.0.0"},
		{"v1.1.0", "http://foo.bar.baz/v1.1.0"},
		{"v1.2.0", "http://foo.bar.baz/v1.2.0"},
	})
}

func TestManageIndex(t *testing.T) {
	require := require.New(t)
	fetcher := newMockFetcher()
	srv := newTestSrv(fetcher)
	fetcher.add("foo", "bar", "v1.0.0", "")
	fetcher.add("foo", "bar", "v1.1.0", "")
	fetcher.add("foo", "baz", "v1.0.0", "")
	fetcher.add("foo", "qux", "v1.0.0", "")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-time.After(20 * time.Millisecond)
		cancel()
	}()
	// in this first call there are no indexed projects, won't have anything to refresh
	srv.ManageIndex(10*time.Millisecond, ctx)

	require.Len(srv.index.projects, 0)

	require.NoError(srv.indexProject("foo", "bar"))
	require.NoError(srv.indexProject("foo", "baz"))
	fetcher.add("foo", "bar", "v1.2.0", "")
	fetcher.add("foo", "baz", "v1.1.0", "")

	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		<-time.After(20 * time.Millisecond)
		cancel()
	}()
	// now there are projects indexed, will refresh those
	srv.ManageIndex(10*time.Millisecond, ctx)
	require.Len(srv.index.projects, 2)
	require.Len(srv.index.projects[newKey("foo", "bar")], 3)
	require.Len(srv.index.projects[newKey("foo", "baz")], 2)
}
