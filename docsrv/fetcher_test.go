package docsrv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testOwner   = "erizocosmico"
	testProject = "test-docsrv"
)

func TestReleases(t *testing.T) {
	require := require.New(t)
	fetcher := newReleaseFetcher("", 1)

	releases, err := fetcher.releases(testOwner, testProject)
	require.NoError(err)

	expected := []string{"v1.0.0", "v1.4.0", "v1.5.0"}
	var result []string
	for _, r := range releases {
		result = append(result, r.tag)
	}

	require.Equal(expected, result)
}
