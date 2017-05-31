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

func main() {
	var (
		apiKey          = os.Getenv("GITHUB_API_KEY")
		org             = os.Getenv("GITHUB_ORG")
		refreshInterval = getRefreshInterval()
	)

	docsrv, err := srv.NewDocSrv(apiKey, org)
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
