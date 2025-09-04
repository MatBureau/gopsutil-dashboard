package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/MatBureau/gopsutil-dashboard/handlers"
	"github.com/MatBureau/gopsutil-dashboard/internal/hashsampler"
)

//go:embed web/*
var webFS embed.FS

func main() {
	mux := http.NewServeMux()

	// --- API ---
	mux.HandleFunc("GET /api/cpu", handlers.CPUHandler)
	mux.HandleFunc("GET /api/mem", handlers.MemHandler)
	mux.HandleFunc("GET /api/disk", handlers.DiskHandler)
	mux.HandleFunc("GET /api/net", handlers.NetHandler)
	mux.HandleFunc("GET /api/host", handlers.HostHandler)
	mux.HandleFunc("GET /api/processes", handlers.ProcessHandler)
	mux.HandleFunc("GET /api/all", handlers.AllHandler)

	// --- Static UI: on "monte" le sous-dossier web/ à la racine ---
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub))) // => / sert web/index.html

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      logMiddleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Contexte "app"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Démarre la boucle toutes les 10s
	hs := hashsampler.Start(ctx, 10*time.Second)

	// Passe le sampler au handler
	handlers.HashSampler = hs

	// Route API
	mux.HandleFunc("GET /api/hash", handlers.HashHandler)

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
