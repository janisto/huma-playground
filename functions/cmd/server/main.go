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
	if err := funcframework.StartHostPort("0.0.0.0", port); err != nil {
		log.Fatalf("start functions framework: %v", err)
	}
}
