package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// handleFile streams an authenticated object from MinIO with HTTP Range
// support so <video> seeking works in the browser.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/files/")
	key, err := url.PathUnescape(key)
	if err != nil || key == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if _, err := s.authenticate(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	info, err := s.obj.Stat(r.Context(), key)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, max-age=86400")

	if rng := r.Header.Get("Range"); rng != "" {
		start, end, ok := parseRange(rng, info.Size)
		if !ok {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size))
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		rc, err := s.obj.OpenRange(ctx, key, start, end)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		defer rc.Close()
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		io.Copy(w, rc)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	rc, err := s.obj.OpenRange(ctx, key, -1, 0)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	io.Copy(w, rc)
}

func parseRange(header string, size int64) (start, end int64, ok bool) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, false
	}
	parts := strings.SplitN(strings.TrimPrefix(header, "bytes="), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	if parts[0] == "" {
		// suffix range: last N bytes
		n, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start >= size {
		return 0, 0, false
	}
	if parts[1] == "" {
		return start, size - 1, true
	}
	end, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil || end < start {
		return 0, 0, false
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true
}