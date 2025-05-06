package main

import (
	"net/http"
)

func main() {
	serveMux := http.NewServeMux()
	handle := http.StripPrefix("/app", http.FileServer(http.Dir("./")))
	serveMux.Handle("/app/", handle)
	serveMux.HandleFunc("/healthz", readiness)
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	server.ListenAndServe()
}

func readiness(rW http.ResponseWriter, req *http.Request) {
	rW.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rW.WriteHeader(200)
	resp := []byte("OK")
	rW.Write(resp)
}
