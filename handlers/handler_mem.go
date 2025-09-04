package handlers

import (
	"net/http"

	"github.com/MatBureau/gopsutil-dashboard/internal/system"
)

func MemHandler(w http.ResponseWriter, r *http.Request) {
	info, err := system.CollectMem(r.Context())
	writeJSON(w, info, err)
}
