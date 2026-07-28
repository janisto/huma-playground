package main

import (
	"cmp"
	"log"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"

	_ "github.com/janisto/huma-playground/functions"
)

func main() {
	port := cmp.Or(os.Getenv("PORT"), "8080")
	host := functionHost(os.Getenv("LOCAL_ONLY"))
	if err := funcframework.StartHostPort(host, port); err != nil {
		log.Fatalf("start functions framework: %v", err)
	}
}

func functionHost(localOnly string) string {
	if localOnly == "true" {
		return "127.0.0.1"
	}
	return ""
}
