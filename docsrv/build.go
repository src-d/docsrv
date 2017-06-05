package docsrv

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/c4milo/unpackit"
)

// buildConfig contains all the configuration passed to the `build docs`
// command to build the docs for a project version.
type buildConfig struct {
	// owner is the name of the organization or user who owns the repository.
	owner string
	// project is the repository name.
	project string
	// version name.
	version string
	// tarballURL is the URL of the .tar.gz with the code of the version.
	tarballURL string
	// baseURL is the base URL for the documentation site. e.g. foo.mydomain.tld/v1.0.0.
	baseURL string
	// hostName is the host name of the documentation site. e.g. foo.mydomain.tld
	hostName string
	// destination is the folder where the documentation should be put once
	// is built.
	destination string
	// sharedFolder will contain all the shared assets needed in the generation.
	sharedFolder string
}

// buildDocs builds the documentation site for the given build configuration.
func buildDocs(conf buildConfig) error {
	start := time.Now()
	resp, err := http.Get(conf.tarballURL)
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
		return fmt.Errorf("error untarring %q: %s", conf.tarballURL, err)
	}

	startBuild := time.Now()
	cmd := exec.Command("make", "docs")
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		"BASE_URL="+conf.baseURL,
		"DESTINATION_PATH="+conf.destination,
		"SHARED_PATH="+conf.sharedFolder,
		"REPOSITORY_NAME="+conf.project,
		"REPOSITORY_OWNER="+conf.owner,
		"VERSION_NAME="+conf.version,
		"HOST_NAME="+conf.hostName,
		"DOCSRV=true",
	)

	logrus.Warnf("make docs: %#v", strings.Join(cmd.Env, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running `make docs` of docs folder at %q: %s. Full error: %s", dir, err, string(output))
	}

	logrus.WithFields(logrus.Fields{
		"project":     conf.project,
		"owner":       conf.owner,
		"version":     conf.version,
		"baseurl":     conf.baseURL,
		"destination": conf.destination,
		"total_time":  fmt.Sprint(time.Since(start)),
		"build_time":  fmt.Sprint(time.Since(startBuild)),
	}).Debugf("build output: %s", string(output))

	if err := os.RemoveAll(dir); err != nil {
		logrus.Warnf("could not delete temp files at %q: %s", dir, err)
	}

	return nil
}
