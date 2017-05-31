package docsrv

import (
	"io/ioutil"
	"testing"

	yaml "gopkg.in/yaml.v1"

	"github.com/stretchr/testify/require"
)

func TestForHost(t *testing.T) {
	require := require.New(t)
	cases := []struct {
		host  string
		ok    bool
		owner string
		repo  string
	}{
		{"foo.bar.baz", true, "foox", "barx"},
		{"foo.bar.haz", false, "", ""},
		{"bar.bar.baz", false, "", ""},
		{"notexists.bar.baz", false, "", ""},
	}
	m := Mappings{
		"foo.bar.baz": "foox/barx",
		"bar.bar.baz": "something",
		"foo.bar.haz": "so/me/th/in/g",
	}

	for _, c := range cases {
		owner, repo, ok := m.forHost(c.host)
		require.Equal(c.ok, ok)
		require.Equal(c.owner, owner)
		require.Equal(c.repo, repo)
	}
}

func TestLoadMappings(t *testing.T) {
	require := require.New(t)
	f, err := ioutil.TempFile("", "mappings")
	require.NoError(err)

	expected := Mappings{
		"foo": "bar/baz",
		"bar": "baz/qux",
	}

	bytes, err := yaml.Marshal(expected)
	require.NoError(err)
	_, err = f.Write(bytes)
	require.NoError(err)

	mappings, err := LoadMappings(f.Name())
	require.NoError(err)

	require.Equal(expected, mappings)
}
