package handlers

import (
	"net/http"

	"github.com/MatBureau/gopsutil-dashboard/internal/system"
)

func NetHandler(w http.ResponseWriter, r *http.Request) {
	includeConnections := r.URL.Query().Get("connections") == "1"
	info, err := system.CollectNet(r.Context(), includeConnections)
	writeJSON(w, info, err)
}
