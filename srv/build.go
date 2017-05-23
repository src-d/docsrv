package srv

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"github.com/Sirupsen/logrus"
	"github.com/c4milo/unpackit"
)

func buildDocs(docsURL, baseURL, destination, sharedFolder string) error {
	resp, err := http.Get(docsURL)
	if err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir("", "docsrv-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %s", err)
	}

	defer resp.Body.Close()
	dir, err := unpackit.Unpack(resp.Body, tmpDir)
	if err != nil {
		return fmt.Errorf("error untarring %q: %s", docsURL, err)
	}

	cmd := exec.Command("make", "docs")
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		"BASE_URL="+baseURL,
		"DESTINATION_FOLDER="+destination,
		"SHARED_FOLDER="+sharedFolder,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running `make build` of docs folder at %q: %s. Full error: %s", dir, err, string(output))
	}

	if err := os.RemoveAll(dir); err != nil {
		logrus.Warnf("could not delete temp files at %q: %s", dir, err)
	}

	return nil
}
