package docsrv

import (
	"io/ioutil"
	"testing"

	toml "github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
)

func TestProjectForHost(t *testing.T) {
	require := require.New(t)
	conf := Config{
		"foo.bar.baz": {"", ""},
		"bar.bar.baz": {"foo", ""},
		"baz.bar.baz": {"foo/bar", ""},
		"qux.bar.baz": {"foo/b/ar", ""},
	}

	cases := []struct {
		host           string
		owner, project string
		ok             bool
	}{
		{"foo.bar.baz", "", "", false},
		{"bar.bar.baz", "", "", false},
		{"baz.bar.baz", "foo", "bar", true},
		{"baz.bar.baz:9090", "foo", "bar", true},
		{"qux.bar.baz", "", "", false},
		{"mux.bar.baz", "", "", false},
	}

	for _, c := range cases {
		owner, project, ok := conf.ProjectForHost(c.host)
		require.Equal(c.ok, ok, "ok does not match %s", c.host)
		require.Equal(c.owner, owner, "owner does not match %s", c.host)
		require.Equal(c.project, project, "project does not match %s", c.host)
	}
}

func TestMinVersionForHost(t *testing.T) {
	require := require.New(t)
	conf := Config{
		"foo.bar.baz": {"", "notaversion"},
		"bar.bar.baz": {"", ""},
		"baz.bar.baz": {"", "v1.0.0"},
	}

	cases := []struct {
		host     string
		expected string
	}{
		{"foo.bar.baz", ""},
		{"bar.bar.baz", ""},
		{"baz.bar.baz", "v1.0.0"},
		{"baz.bar.baz:9090", "v1.0.0"},
		{"qux.bar.baz", ""},
	}

	for _, c := range cases {
		cv := newVersion(c.expected)
		require.Equal(cv, conf.MinVersionForHost(c.host), c.host)
	}
}

func TestLoadConfig(t *testing.T) {
	require := require.New(t)
	f, err := ioutil.TempFile("", "config")
	require.NoError(err)
	defer f.Close()

	expected := Config{
		"foo.bar.baz": {"bar/baz", "v1.0.0"},
		"bar.bar.baz": {"bar/bar", "v1.1.0"},
	}

	require.NoError(toml.NewEncoder(f).Encode(expected))

	config, err := LoadConfig(f.Name())
	require.NoError(err)

	require.Equal(expected, config)
}

func TestStripPort(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"foo.bar.baz", "foo.bar.baz"},
		{"foo.bar.baz:9090", "foo.bar.baz"},
	}

	for _, c := range cases {
		require.Equal(t, c.out, stripPort(c.in), c.in)
	}
}
