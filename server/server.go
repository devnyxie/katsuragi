// Package server exposes katsuragi's extraction and screenshot
// capabilities as an HTTP API. It is a plain importable package, not just
// a cmd/server-only concern: a consuming application can call New and
// either run ListenAndServe itself (passing its own port and pool
// configuration) or mount Handler into its own http.Server/mux instead of
// shelling out to a separate binary.
//
// It calls the browser/screenshot packages synchronously - no job queue,
// no separate worker process - which is enough to develop against and to
// serve a frontend once one exists. A queue/worker split can be
// introduced later without changing the route contract.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/devnyxie/katsuragi"
	"github.com/devnyxie/katsuragi/browser"
)

// Config configures a Server. All fields are optional; zero values fall
// back to sane defaults (see setDefaults).
type Config struct {
	// Port is the TCP port ListenAndServe binds to. Ignored if the
	// caller only uses Handler(). Default 8080.
	Port int

	// BrowserPoolSize is the number of warm Chrome instances to keep
	// running - see browser.PoolConfig.Size. Default 2.
	BrowserPoolSize int
	// BrowserMaxTabs bounds concurrent tabs per Chrome instance - see
	// browser.PoolConfig.MaxTabsPerBrowser. Default 4.
	BrowserMaxTabs int
	// Stealth controls anti-bot-detection depth - see
	// browser.PoolConfig.Stealth. Default browser.StealthBasic.
	Stealth browser.StealthLevel
	// Supervised, if true, launches Chrome under the orphan-preventing
	// supervisor process - see browser.PoolConfig.Supervised. Default
	// false.
	Supervised bool
	// Proxy, if set, routes browser traffic through an upstream proxy -
	// see browser.PoolConfig.Proxy.
	Proxy *browser.ProxyConfig

	// FetcherProps configures the static-HTML extraction Fetcher used by
	// /v1/extract - see katsuragi.FetcherProps. The zero value is valid
	// and uses katsuragi's own defaults.
	FetcherProps katsuragi.FetcherProps

	// ShutdownTimeout bounds how long ListenAndServe's graceful shutdown
	// waits for in-flight requests before giving up. Default 15s.
	ShutdownTimeout time.Duration
}

func (c *Config) setDefaults() {
	if c.Port <= 0 {
		c.Port = 8080
	}
	if c.BrowserPoolSize <= 0 {
		c.BrowserPoolSize = 2
	}
	if c.BrowserMaxTabs <= 0 {
		c.BrowserMaxTabs = 4
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 15 * time.Second
	}
}

// Server is a running (or ready-to-run) katsuragi HTTP API: a static-HTML
// Fetcher plus a warm browser.Pool, wired to the /healthz, /v1/extract,
// and /v1/screenshot routes.
type Server struct {
	cfg     Config
	fetcher *katsuragi.Fetcher
	pool    *browser.Pool
	mux     *http.ServeMux
	http    *http.Server
}

// New builds a Server: constructs the Fetcher and launches cfg's browser
// pool. The pool is already warm (Chrome processes started) by the time
// New returns successfully. Call Close when done with it, whether or not
// ListenAndServe was ever called.
func New(ctx context.Context, cfg Config) (*Server, error) {
	cfg.setDefaults()

	fetcher := katsuragi.NewFetcher(&cfg.FetcherProps)

	pool, err := browser.NewPool(ctx, browser.PoolConfig{
		Size:              cfg.BrowserPoolSize,
		MaxTabsPerBrowser: cfg.BrowserMaxTabs,
		Stealth:           cfg.Stealth,
		Supervised:        cfg.Supervised,
		Proxy:             cfg.Proxy,
	})
	if err != nil {
		return nil, fmt.Errorf("server: failed to start browser pool: %w", err)
	}

	s := &Server{cfg: cfg, fetcher: fetcher, pool: pool}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /v1/extract", s.handleExtract)
	mux.HandleFunc("GET /v1/screenshot", s.handleScreenshot)
	s.mux = mux

	return s, nil
}

// Handler returns the API's http.Handler, for a consuming application
// that wants to mount these routes into its own http.Server or mux
// rather than calling ListenAndServe.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// ListenAndServe binds cfg.Port and serves until ctx is cancelled, then
// gracefully shuts down (bounded by cfg.ShutdownTimeout). It does not
// call Close - the caller owns the Server (and its browser pool) after
// ListenAndServe returns, and should call Close explicitly.
func (s *Server) ListenAndServe(ctx context.Context) error {
	s.http = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Port),
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.http.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	}
}

// Close tears down the underlying browser pool. Safe to call once,
// whether or not ListenAndServe was ever used.
func (s *Server) Close() error {
	return s.pool.Close()
}
