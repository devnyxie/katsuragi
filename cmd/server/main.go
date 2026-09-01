// Command server is a thin runnable wrapper around the importable
// katsuragi/server package: it reads configuration from the environment
// and starts an HTTP API exposing katsuragi's extraction and screenshot
// capabilities. The actual server logic lives in katsuragi/server so a
// consuming application can import and embed it directly instead of
// shelling out to this binary - see that package's doc comment.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/devnyxie/katsuragi/browser"
	"github.com/devnyxie/katsuragi/server"
)

func main() {
	cfg := server.Config{
		Port:            getEnvInt("PORT", 8080),
		BrowserPoolSize: getEnvInt("BROWSER_POOL_SIZE", 2),
		BrowserMaxTabs:  getEnvInt("BROWSER_MAX_TABS", 4),
		Stealth:         parseStealthLevel(getEnv("STEALTH_LEVEL", "basic")),
		Supervised:      getEnv("SUPERVISED", "true") == "true",
	}

	log.Printf("starting browser pool (size=%d, maxTabs=%d, stealth=%v, supervised=%v)...",
		cfg.BrowserPoolSize, cfg.BrowserMaxTabs, cfg.Stealth, cfg.Supervised)

	srv, err := server.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
	defer srv.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("katsuragi server listening on :%d", cfg.Port)
	if err := srv.ListenAndServe(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
	log.Println("shut down cleanly")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func parseStealthLevel(v string) browser.StealthLevel {
	switch v {
	case "off":
		return browser.StealthOff
	case "headful":
		return browser.StealthHeadful
	default:
		return browser.StealthBasic
	}
}
