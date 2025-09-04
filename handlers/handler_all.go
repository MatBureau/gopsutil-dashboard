package handlers

import (
	"net/http"

	"github.com/MatBureau/gopsutil-dashboard/internal/system"
)

func AllHandler(w http.ResponseWriter, r *http.Request) {
	info, err := system.CollectAll(r.Context())
	writeJSON(w, info, err)
}
