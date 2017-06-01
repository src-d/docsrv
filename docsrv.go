package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/src-d/docsrv/docsrv"
)

const (
	sharedFolder = "/etc/shared"
	baseFolder   = "/var/www/public"
	mappingsFile = "/etc/docsrv/mappings.yml"
)

func main() {
	var (
		apiKey          = os.Getenv("GITHUB_API_KEY")
		defaultOwner    = os.Getenv("GITHUB_ORG")
		debug           = os.Getenv("DEBUG_LOG") != ""
		refreshToken    = os.Getenv("REFRESH_TOKEN")
		refreshInterval = getRefreshInterval()
	)

	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	mappings, err := docsrv.LoadMappings(mappingsFile)
	if err != nil {
		logrus.Fatalf("unable to load mappings: %s", err)
	}

	docsrv := docsrv.New(docsrv.Options{
		GitHubAPIKey: apiKey,
		DefaultOwner: defaultOwner,
		BaseFolder:   baseFolder,
		SharedFolder: sharedFolder,
		RefreshToken: refreshToken,
		Mappings:     mappings,
	})
	if err != nil {
		logrus.Fatalf("unable to start a new docsrv: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go docsrv.ManageIndex(refreshInterval, ctx)
	defer cancel()

	server := &http.Server{
		Addr:         ":9091",
		Handler:      docsrv,
		WriteTimeout: 5 * time.Minute,
		ReadTimeout:  1 * time.Minute,
	}

	if err := server.ListenAndServe(); err != nil {
		logrus.Fatal(err)
	}
}

func getRefreshInterval() time.Duration {
	n, err := strconv.Atoi(os.Getenv("DOCSRV_REFRESH"))
	if err != nil || n < 1 {
		n = 5
	}

	return time.Duration(n) * time.Minute
}
