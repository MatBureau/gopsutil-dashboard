package handlers

import (
	"net/http"

	"github.com/MatBureau/gopsutil-dashboard/internal/system"
)

func HostHandler(w http.ResponseWriter, r *http.Request) {
	info, err := system.CollectHost(r.Context())
	writeJSON(w, info, err)
}
