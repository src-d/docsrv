package srv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testOwner   = "erizocosmico"
	testProject = "test-docsrv"
)

func TestGitHubReleases(t *testing.T) {
	require := require.New(t)
	github := NewGitHub("")

	releases, err := github.Releases(testOwner, testProject)
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
	github := NewGitHub("")

	release, err := github.Release(testOwner, testProject, "v1.6.0")
	require.Error(err)

	release, err = github.Release(testOwner, testProject, "v1.4.0")
	require.NoError(err)
	require.Equal("v1.4.0", release.Tag)
}

func TestGitHubLatest(t *testing.T) {
	require := require.New(t)
	github := NewGitHub("")

	release, err := github.Latest(testOwner, testProject)
	require.NoError(err)
	require.Equal("v1.5.0", release.Tag)
}
