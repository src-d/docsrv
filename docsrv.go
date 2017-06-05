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
	configFile   = "/etc/docsrv/conf.d/config.toml"
)

func main() {
	var (
		apiKey          = os.Getenv("GITHUB_API_KEY")
		debug           = os.Getenv("DEBUG_LOG") != ""
		refreshToken    = os.Getenv("REFRESH_TOKEN")
		refreshInterval = getRefreshInterval()
	)

	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	config, err := docsrv.LoadConfig(configFile)
	if err != nil {
		logrus.Fatalf("unable to load config: %s", err)
	}

	if len(config) == 0 {
		logrus.Fatalf("there are no hosts configured in %s", configFile)
	}

	docsrv := docsrv.New(docsrv.Options{
		GitHubAPIKey: apiKey,
		BaseFolder:   baseFolder,
		SharedFolder: sharedFolder,
		RefreshToken: refreshToken,
		Config:       config,
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
