package main

import (
	"net/http"
)

func main() {
	serveMux := http.NewServeMux()
	handle := http.FileServer(http.Dir("./"))
	serveMux.Handle("/", handle)
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	server.ListenAndServe()
}
