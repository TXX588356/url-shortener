package handlers

import (
	"errors"
	"leetx/url/shortener/utils"
	"net/http"
)

func (h *URLHandler) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("code")

	err := h.service.Delete(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, utils.ErrURLNotFound) {
			http.Error(w, "no short code found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete short code", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
