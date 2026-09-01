//go:build integration

package browser

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// startTestProxy runs a minimal authenticated forward proxy: it accepts
// one connection at a time, validates HTTP Basic Proxy-Authorization,
// responds 407 if missing/wrong, and otherwise dials the requested host
// and relays bytes. Handles both CONNECT (tunneling, used by Chrome for
// https:// targets) and plain forward requests (used for http:// targets,
// which is what this test exercises against an httptest.Server).
func startTestProxy(t *testing.T, username, password string) (addr string, requestsSeen func() int) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for test proxy: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	var count atomic.Int64

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			count.Add(1)
			go handleProxyConn(t, conn, expectedAuth)
		}
	}()

	return ln.Addr().String(), func() int { return int(count.Load()) }
}

func handleProxyConn(t *testing.T, conn net.Conn, expectedAuth string) {
	defer conn.Close()

	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	if req.Header.Get("Proxy-Authorization") != expectedAuth {
		fmt.Fprint(conn, "HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"test\"\r\nContent-Length: 0\r\n\r\n")
		return
	}

	target, err := net.Dial("tcp", req.Host)
	if err != nil {
		fmt.Fprint(conn, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
		return
	}
	defer target.Close()

	if req.Method == http.MethodConnect {
		fmt.Fprint(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		go func() { _, _ = io.Copy(target, br) }()
		_, _ = io.Copy(conn, target)
		return
	}

	req.Header.Del("Proxy-Authorization")
	req.Header.Del("Proxy-Connection")
	if err := req.Write(target); err != nil {
		return
	}
	_, _ = io.Copy(conn, target)
}

func TestIntegration_ProxyAuth_Succeeds(t *testing.T) {
	const user, pass = "testuser", "testpass123"
	proxyAddr, requestsSeen := startTestProxy(t, user, pass)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>ViaProxy</title></head><body></body></html>")
	}))
	defer server.Close()

	pool := newTestPool(t, PoolConfig{
		Size:              1,
		MaxTabsPerBrowser: 1,
		LaunchTimeout:     30 * time.Second,
		Proxy: &ProxyConfig{
			Server:   "http://" + proxyAddr,
			Username: user,
			Password: pass,
		},
	})

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	var title string
	if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL), chromedp.Title(&title)); err != nil {
		t.Fatalf("navigate through proxy failed: %v", err)
	}
	if title != "ViaProxy" {
		t.Fatalf("expected title %q, got %q", "ViaProxy", title)
	}
	if requestsSeen() == 0 {
		t.Fatalf("expected the request to go through the proxy, but the proxy saw no connections")
	}
}

func TestIntegration_ProxyAuth_WrongCredentialsFail(t *testing.T) {
	proxyAddr, _ := startTestProxy(t, "realuser", "realpass")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>ViaProxy</title></head><body></body></html>")
	}))
	defer server.Close()

	pool := newTestPool(t, PoolConfig{
		Size:              1,
		MaxTabsPerBrowser: 1,
		LaunchTimeout:     30 * time.Second,
		Proxy: &ProxyConfig{
			Server:   "http://" + proxyAddr,
			Username: "wronguser",
			Password: "wrongpass",
		},
	})

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	ctx, cancel := context.WithTimeout(tab.Ctx(), 15*time.Second)
	defer cancel()

	var title string
	err = chromedp.Run(ctx, chromedp.Navigate(server.URL), chromedp.Title(&title))
	if err == nil {
		t.Fatalf("expected navigation to fail with wrong proxy credentials, got title %q with no error", title)
	}
}
