package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

func main() {
	cfg := apiConfig{}
	serveMux := http.NewServeMux()
	handle := http.StripPrefix("/app", http.FileServer(http.Dir("./")))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(handle))
	serveMux.HandleFunc("/healthz", readiness)
	serveMux.HandleFunc("/metrics", cfg.metrics)
	serveMux.HandleFunc("/reset", cfg.reset)
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

func (cfg *apiConfig) metrics(rW http.ResponseWriter, req *http.Request) {
	rW.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rW.WriteHeader(200)
	respStr := fmt.Sprintf("Hits: %d", cfg.serverHits.Load())
	resp := []byte(respStr)
	rW.Write(resp)
}

func (cfg *apiConfig) reset(rW http.ResponseWriter, req *http.Request) {
	rW.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rW.WriteHeader(200)
	cfg.serverHits.Store(0)
	respStr := fmt.Sprintf("Hits: %d", cfg.serverHits.Load())
	resp := []byte(respStr)
	rW.Write(resp)
}

type apiConfig struct {
	serverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.serverHits.Add(1)
		next.ServeHTTP(w, r)
	})

}
