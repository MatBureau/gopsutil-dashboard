package handlers

import (
	"net/http"

	"github.com/MatBureau/gopsutil-dashboard/internal/system"
)

func DiskHandler(w http.ResponseWriter, r *http.Request) {
	info, err := system.CollectDisk(r.Context())
	writeJSON(w, info, err)
}
