package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devnyxie/katsuragi"
	"github.com/devnyxie/katsuragi/browser"
	"github.com/devnyxie/katsuragi/screenshot"
)

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// extractResponse is GET /v1/extract's response shape. Per-field errors are
// reported alongside any fields that did succeed, rather than failing the
// whole request if e.g. only favicons couldn't be found.
type extractResponse struct {
	URL         string            `json:"url"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Favicons    []string          `json:"favicons,omitempty"`
	Links       []string          `json:"links,omitempty"`
	Errors      map[string]string `json:"errors,omitempty"`
}

// GET /v1/extract?url=...&fields=title,description,favicons,links&category=all
func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target := q.Get("url")
	if target == "" {
		writeError(w, http.StatusBadRequest, "missing required query param: url")
		return
	}

	fields := parseFields(q.Get("fields"))

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	resp := extractResponse{URL: target, Errors: map[string]string{}}

	if fields["title"] {
		if v, err := s.fetcher.GetTitle(ctx, target); err != nil {
			resp.Errors["title"] = err.Error()
		} else {
			resp.Title = v
		}
	}
	if fields["description"] {
		if v, err := s.fetcher.GetDescription(ctx, target); err != nil {
			resp.Errors["description"] = err.Error()
		} else {
			resp.Description = v
		}
	}
	if fields["favicons"] {
		if v, err := s.fetcher.GetFavicons(ctx, target); err != nil {
			resp.Errors["favicons"] = err.Error()
		} else {
			resp.Favicons = v
		}
	}
	if fields["links"] {
		category := q.Get("category")
		if v, err := s.fetcher.GetLinks(ctx, katsuragi.GetLinksProps{Url: target, Category: category}); err != nil {
			resp.Errors["links"] = err.Error()
		} else {
			resp.Links = v
		}
	}
	if len(resp.Errors) == 0 {
		resp.Errors = nil
	}

	writeJSON(w, http.StatusOK, resp)
}

func parseFields(raw string) map[string]bool {
	if raw == "" {
		return map[string]bool{"title": true, "description": true, "favicons": true, "links": true}
	}
	fields := map[string]bool{}
	for _, f := range strings.Split(raw, ",") {
		fields[strings.TrimSpace(f)] = true
	}
	return fields
}

// GET /v1/screenshot?url=...&width=1280&height=800&fullPage=true&format=png
//
//	&waitUntil=networkidle|selector|fully_loaded&waitSelector=...&timeoutMs=15000
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target := q.Get("url")
	if target == "" {
		writeError(w, http.StatusBadRequest, "missing required query param: url")
		return
	}

	width := queryInt(q, "width", 1280)
	height := queryInt(q, "height", 800)
	fullPage := q.Get("fullPage") == "true"
	format := screenshot.FormatPNG
	contentType := "image/png"
	if q.Get("format") == "jpeg" {
		format = screenshot.FormatJPEG
		contentType = "image/jpeg"
	}
	timeoutMs := queryInt(q, "timeoutMs", 15000)
	timeout := time.Duration(timeoutMs) * time.Millisecond

	var waitConfig browser.WaitConfig
	switch q.Get("waitUntil") {
	case "selector":
		selector := q.Get("waitSelector")
		if selector == "" {
			writeError(w, http.StatusBadRequest, "waitUntil=selector requires waitSelector")
			return
		}
		waitConfig = browser.WaitForSelector(selector, timeout)
	case "fully_loaded":
		waitConfig = browser.WaitFullyLoaded(timeout)
	default:
		waitConfig = browser.WaitUntilNetworkIdle(timeout)
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout+30*time.Second)
	defer cancel()

	result, err := screenshot.Capture(ctx, s.pool, screenshot.Request{
		URL:        target,
		WaitConfig: waitConfig,
		Viewport:   screenshot.Viewport{Width: int64(width), Height: int64(height)},
		FullPage:   fullPage,
		Format:     format,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Katsuragi-Partial", strconv.FormatBool(result.Partial))
	w.Header().Set("X-Katsuragi-Duration-Ms", strconv.FormatInt(result.DurationMs, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.ImageBytes)
}

func queryInt(q url.Values, key string, def int) int {
	v := q.Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
