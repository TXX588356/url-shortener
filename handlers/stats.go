package handlers

import (
	"encoding/json"
	"errors"
	"leetx/url/shortener/utils"
	"net/http"
)

func (h *URLHandler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("code")

	clickCount, err := h.service.GetClickCount(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, utils.ErrURLNotFound) {
			http.Error(w, "no short code found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to retrieve click count", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(
		map[string]any{
			"short_code":  shortCode,
			"click_count": clickCount,
		},
	)
}
