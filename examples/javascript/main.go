package main

import (
	"net/http"
)

func main() {
	// Simple static webserver:
	logger.Fatal(http.ListenAndServe(":1999", http.FileServer(http.Dir("./"))))
}
