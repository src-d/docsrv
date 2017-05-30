package main

import (
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/src-d/docsrv/docsrv"
)

func main() {
	var (
		apiKey = os.Getenv("GITHUB_API_KEY")
		org    = os.Getenv("GITHUB_ORG")
	)

	docsrv, err := srv.NewDocSrv(apiKey, org)
	if err != nil {
		logrus.Fatalf("unable to start a new docsrv: %s", err)
	}

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
