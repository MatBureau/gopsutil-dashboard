package handlers

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/MatBureau/gopsutil-dashboard/internal/hashsampler"
)

var HashSampler *hashsampler.Sampler

type hashResp struct {
	RandHash     string `json:"randhash"`
	HashBytesHex string `json:"hashbytes_hex"`
	UpdatedAt    string `json:"updatedAt"`
	Warning      string `json:"warning,omitempty"`
}

func HashHandler(w http.ResponseWriter, r *http.Request) {
	if HashSampler == nil {
		writeJSON(w, nil, fmt.Errorf("hash sampler not initialized"))
		return
	}

	v, ts, err := HashSampler.Snapshot()
	resp := hashResp{UpdatedAt: ts.Format(time.RFC3339)}

	if v != nil {
		resp.RandHash = v.HashString
		resp.HashBytesHex = hex.EncodeToString(v.HashBytes)
	}
	if err != nil {
		resp.Warning = err.Error()
	}
	writeJSON(w, resp, nil)
}
