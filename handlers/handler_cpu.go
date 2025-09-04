package handlers

import (
	"net/http"

	"github.com/MatBureau/gopsutil-dashboard/internal/system"
)

func CPUHandler(w http.ResponseWriter, r *http.Request) {
	info, err := system.CollectCPU(r.Context())
	writeJSON(w, info, err)
}
