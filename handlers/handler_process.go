package handlers

import (
	"net/http"
	"strconv"

	"github.com/MatBureau/gopsutil-dashboard/internal/system"
)

func ProcessHandler(w http.ResponseWriter, r *http.Request) {
	topN := 15
	if n := r.URL.Query().Get("top"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v > 0 {
			topN = v
		}
	}
	info, err := system.CollectProcesses(r.Context(), topN)
	writeJSON(w, info, err)
}
