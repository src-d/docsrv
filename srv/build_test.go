package srv

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDocs(t *testing.T) {
	require := require.New(t)
	url, close := tarGzServer()
	defer close()

	tmpDir, err := ioutil.TempDir("", "docsrv-test-")
	require.NoError(err)

	require.NoError(buildDocs(url, "http://foo.bar", tmpDir))
	assertMakefileOutput(t, tmpDir, "http://foo.bar")
}
