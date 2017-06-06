package docsrv

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testOwner   = "erizocosmico"
	testProject = "test-docsrv"
)

func TestReleases(t *testing.T) {
	apiKey := os.Getenv("GITHUB_API_KEY")
	require := require.New(t)
	fetcher := newReleaseFetcher(apiKey, 1)

	releases, err := fetcher.releases(testOwner, testProject, newVersion("v1.4.0"))
	require.NoError(err)

	expected := []string{"v1.4.0", "v1.5.0"}
	var result []string
	for _, r := range releases {
		result = append(result, r.tag)
	}

	require.Equal(expected, result)
}
