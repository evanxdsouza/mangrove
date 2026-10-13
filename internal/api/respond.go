package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/evanxdsouza/mangrove/internal/store"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeStoreError maps the store errors a client can cause -- not found,
// a duplicate slug, a reference to a parent that doesn't exist -- to 4xx
// instead of a 500 carrying raw SQLite text.
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case store.IsDuplicate(err):
		msg := "already exists"
		if errors.Is(err, store.ErrDuplicate) && err.Error() != store.ErrDuplicate.Error() {
			msg = strings.TrimSuffix(err.Error(), ": "+store.ErrDuplicate.Error())
		} else if strings.Contains(err.Error(), ".slug") {
			msg = "slug is already in use"
		}
		writeError(w, http.StatusConflict, msg)
	case store.IsMissingReference(err):
		writeError(w, http.StatusNotFound, "referenced resource does not exist")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

// maxRequestBody bounds every /api request body. Nothing the API accepts
// comes anywhere near it (the largest is an env var value, e.g. a PEM
// certificate); without it, even the unauthenticated login endpoint would
// buffer an arbitrarily large JSON string into memory on a small box.
const maxRequestBody = 5 << 20

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
