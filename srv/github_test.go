package srv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitHubReleases(t *testing.T) {
	require := require.New(t)
	github := NewGitHub("", "erizocosmico")

	releases, err := github.Releases("test-docsrv")
	require.NoError(err)

	expected := []string{"v1.0.0", "v1.4.0", "v1.5.0"}
	var result []string
	for _, r := range releases {
		result = append(result, r.Tag)
	}

	require.Equal(expected, result)
}

func TestGitHubRelease(t *testing.T) {
	require := require.New(t)
	github := NewGitHub("", "erizocosmico")

	release, err := github.Release("test-docsrv", "v1.6.0")
	require.Error(err)

	release, err = github.Release("test-docsrv", "v1.4.0")
	require.NoError(err)
	require.Equal("v1.4.0", release.Tag)
}

func TestGitHubLatest(t *testing.T) {
	require := require.New(t)
	github := NewGitHub("", "erizocosmico")

	release, err := github.Latest("test-docsrv")
	require.NoError(err)
	require.Equal("v1.5.0", release.Tag)
}
