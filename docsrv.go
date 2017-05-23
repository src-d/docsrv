package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/src-d/docsrv/srv"
)

func main() {
	var (
		apiKey = os.Getenv("GITHUB_API_KEY")
		org    = os.Getenv("GITHUB_ORG")
	)

	server := &http.Server{
		Addr:         ":9091",
		Handler:      srv.NewDocSrv(apiKey, org),
		WriteTimeout: 5 * time.Minute,
		ReadTimeout:  1 * time.Minute,
	}

	log.Fatal(server.ListenAndServe())
}
