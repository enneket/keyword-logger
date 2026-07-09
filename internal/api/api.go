package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"keyword-logger/internal/counter"
)

const timeFormat = "2006-01-02T15:04"

//go:embed templates/index.html
var indexHTML []byte

//go:embed templates/icon.svg
var faviconSVG []byte

type Server struct {
	counter *counter.Counter
	server  *http.Server
}

func New(port int, c *counter.Counter) *Server {
	s := &Server{counter: c}
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/summary", s.handleSummary)
	mux.HandleFunc("/favicon.svg", s.handleFavicon)
	mux.HandleFunc("/app", s.handleIndex)
	mux.HandleFunc("/", s.handleIndex)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}
	return s
}

func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	return s.server.Close()
}

func parseTimeParam(r *http.Request, name string) (time.Time, bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(timeFormat, v, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	app := r.URL.Query().Get("app")
	since, _ := parseTimeParam(r, "since")
	until, _ := parseTimeParam(r, "until")
	gran := r.URL.Query().Get("granularity")
	if gran != "" && !counter.IsValidGranularity(gran) {
		http.Error(w, "invalid granularity: must be hour|day|week|month|year", http.StatusBadRequest)
		return
	}

	stats := s.counter.GetStats(since, until, app, gran)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(indexHTML)
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(faviconSVG)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	stats := s.counter.GetStats(time.Time{}, time.Time{}, "", "")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "Total keystrokes: %d\n", stats.Total)
	fmt.Fprintf(w, "Since: %s\n", stats.StartedAt)
	fmt.Fprintf(w, "Apps tracked: %d\n\n", len(stats.Apps))

	for appName, appStats := range stats.Apps {
		fmt.Fprintf(w, "  %s: %d\n", appName, appStats.Total)
	}
}
