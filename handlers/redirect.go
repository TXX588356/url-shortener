package handlers

import (
	"errors"
	"leetx/url/shortener/utils"
	"net/http"
)

func (h *URLHandler) ExtractHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("code")

	originalURL, err := h.service.ResolveURL(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, utils.ErrURLExpired) {
			http.Error(w, "URL has expired", http.StatusGone)
			return
		}

		if errors.Is(err, utils.ErrURLNotFound) {
			http.Error(w, "no url found", http.StatusNotFound)
			return
		}

		http.Error(w, "failed to retrieve original URL", http.StatusInternalServerError)
		return

	}

	http.Redirect(w, r, originalURL, http.StatusFound)
}
