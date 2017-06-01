package docsrv

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

	conf := buildConfig{
		tarballURL:   url,
		baseURL:      "http://foo.bar",
		destination:  tmpDir,
		sharedFolder: "/etc/shared",
		project:      "src-d",
		owner:        "docsrv",
		version:      "v1.2.3",
	}
	require.NoError(buildDocs(conf))
	assertMakefileOutput(t, tmpDir, conf.baseURL, conf.project, conf.owner, conf.version)
}
