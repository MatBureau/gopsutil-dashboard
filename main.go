package main

import (
	"embed"
	"log"
	"net/http"
	"time"

	"github.com/MatBureau/gopsutil-dashboard/handlers"
)

var webFS embed.FS

func main() {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("GET /api/cpu", handlers.CPUHandler)
	mux.HandleFunc("GET /api/mem", handlers.MemHandler)
	mux.HandleFunc("GET /api/disk", handlers.DiskHandler)
	mux.HandleFunc("GET /api/net", handlers.NetHandler)
	mux.HandleFunc("GET /api/host", handlers.HostHandler)
	mux.HandleFunc("GET /api/processes", handlers.ProcessHandler)
	mux.HandleFunc("GET /api/all", handlers.AllHandler)

	// Static UI (index.html, app.js, styles.css)
	fs := http.FileServer(http.FS(webFS))
	mux.Handle("/", http.StripPrefix("/", fs))

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      logMiddleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Serving on http://localhost:8080")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
