# katsuragi

![Go Build](https://github.com/devnyxie/katsuragi/actions/workflows/go.yml/badge.svg)
[![codecov](https://codecov.io/github/devnyxie/katsuragi/branch/main/graph/badge.svg?token=XFRMNJA858)](https://codecov.io/github/devnyxie/katsuragi)

A Go toolkit for web content processing, analysis, and SEO optimization, offering utilities to efficiently extract favicons, links, descriptions and titles. 

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**

- [Features](#features)
- [Repository Layout](#repository-layout)
- [Installation](#installation)
- [Usage](#usage)
  - [Title](#title)
  - [Description](#description)
  - [Favicons](#favicons)
  - [Links/Backlinks](#linksbacklinks)
  - [Screenshots](#screenshots)
    - [Avoiding bot detection](#avoiding-bot-detection)
    - [Routing through a proxy](#routing-through-a-proxy)
    - [Solving CAPTCHAs (Cloudflare Turnstile)](#solving-captchas-cloudflare-turnstile)
  - [HTTP API Server](#http-api-server)
- [Local Development](#local-development)
  - [Testing](#testing)
  - [Integration Testing](#integration-testing)
  - [Code Coverage](#code-coverage)
- [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Features

- LRU Caching
- Timeout
- User-Agent
- Request retries with backoff, and request coalescing for concurrent duplicate fetches
- Headless-browser screenshots via a warm, reusable Chrome pool (`browser`/`screenshot` packages), with layered wait-for-load strategies for pages that are slow or dynamic to render
- Anti-bot stealth, proxy support, and CAPTCHA solving for the browser pool
- An importable HTTP API (`server` package) exposing extraction and screenshots over REST, plus a ready-to-run binary (`cmd/server`)

# Repository Layout

katsuragi is a Go library first: every capability is an importable package, and `cmd/server` is a thin binary built on top of them, not the other way around.

| Path | What it is |
|---|---|
| *(root package)* | Static-HTML extraction: `GetTitle`/`GetDescription`/`GetFavicons`/`GetLinks` |
| `browser/` | The warm Chrome pool: tabs, wait strategies, stealth, proxy, the orphan-preventing supervisor |
| `screenshot/` | Screenshot capture built on `browser/`, plus Turnstile CAPTCHA detection/solving |
| `captcha/` | Provider-agnostic CAPTCHA solver interface (`capsolver.com` implementation included) |
| `server/` | The HTTP API as an importable package — mount it into your own app, or run it standalone |
| `cmd/server/` | A runnable binary wrapping `server`, configured entirely via environment variables |
| `web/` | Reserved for a future client webapp frontend; empty for now |

# Installation

```bash
go get github.com/devnyxie/katsuragi
```

# Usage

## Title

The GetTitle() function currently supports the following title meta tags:

- `<title>Title</title>`
- `<meta name="twitter:title" content="Title">`
- `<meta property="og:title" content="Title">`

```go
import (
	"context"

	. "github.com/devnyxie/katsuragi"
)

func main() {
  // Create a new fetcher with a timeout of 3 seconds and a cache capacity of 10
  fetcher := NewFetcher(
    &FetcherProps{
      Timeout:       3000, // 3 seconds
      CacheCap: 10, // 10 Network Requests will be cached
    },
  )

  defer fetcher.ClearCache()

  // Get website's title
  title, err := fetcher.GetTitle(context.Background(), "https://www.example.com")
}
```

## Description

The GetDescription() function currently supports the following description meta tags:

- `<meta name="description" content="Description">`
- `<meta name="twitter:description" content="Description">`
- `<meta property="og:description" content="Description">`

```go
...
  // Get website's description
  description, err := fetcher.GetDescription(context.Background(), "https://www.example.com")
...
```

## Favicons

The GetFavicons() function currently supports the following favicon meta tags:

- `<link rel="icon" href="favicon.ico">`
- `<link rel="apple-touch-icon" href="favicon.png">`
- `<meta property="og:image" content="favicon.png">`
  > Open Graph image (`og:image`) will be used only if both `og:image:width` and `og:image:height` are present and equal, forming a square image.

```go
...
  // Get website's favicons
  favicons, err := fetcher.GetFavicons(context.Background(), "https://www.example.com")
  // [https://www.example.com/favicon.ico, https://www.example.com/favicon.png]
...
```

## Links/Backlinks

The GetLinks() function searches for all `<a>` tags in the HTML document and returns a slice of links.

Options:

- `Url` (required): The URL of the website to fetch.
- `Category` (optional): The category of links to fetch. Possible values are `internal`, `external`, and `all`. Default is `all`.

```go
  // Get website's links
  links, err := fetcher.GetLinks(context.Background(), GetLinksProps{
    Url: "https://www.example.com",
    Category: "external",
  })
  // [https://www.youtube.com/example, https://www.facebook.com/example]
```

## Screenshots

The `browser` and `screenshot` packages let you render pages with a real (headless) Chrome and capture a screenshot — useful for JS-heavy pages.

`browser.Pool` keeps a fixed number of Chrome processes warm so requests don't pay browser-launch cold-start cost, hands out tabs bounded by a pool-wide concurrency limit, and recycles/health-checks instances in the background. One `Pool` is meant to live for the lifetime of your process; requires a Chrome/Chromium binary to be installed and discoverable (or set `chromedp`'s usual env vars to point at one).

```go
import (
	"context"
	"time"

	"github.com/devnyxie/katsuragi/browser"
	"github.com/devnyxie/katsuragi/screenshot"
)

func main() {
	pool, err := browser.NewPool(context.Background(), browser.PoolConfig{
		Size:              2, // warm Chrome processes
		MaxTabsPerBrowser: 4, // concurrent tabs per process
	})
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	result, err := screenshot.Capture(context.Background(), pool, screenshot.Request{
		URL: "https://www.example.com",
		// WaitConfig controls how long, and by what criteria, Capture waits
		// for the page to be considered ready before screenshotting.
		WaitConfig: browser.WaitFullyLoaded(15 * time.Second),
		Viewport:   screenshot.Viewport{Width: 1280, Height: 800},
		FullPage:   true,
		Format:     screenshot.FormatPNG,
	})
	if err != nil {
		panic(err)
	}

	// result.ImageBytes is the encoded image; result.Partial is true if the
	// page never fully settled within the wait timeout (a best-effort
	// screenshot was still captured rather than erroring out).
	_ = result
}
```

Wait strategies (composable via `browser.WaitConfig`, or use a preset):

- `browser.WaitUntilNetworkIdle(timeout)` — DOM readiness + no in-flight network requests for a quiet window. Good default for most pages.
- `browser.WaitForSelector(selector, timeout)` — additionally waits for a CSS selector you know the page will render.
- `browser.WaitFullyLoaded(timeout)` — the above plus scrolling to settle lazy-loaded images. Slower, meant for pages known to be hard to render.

One `browser.Pool` represents one process's worth of browsers; there's no cross-process coordination. To scale beyond one process, run multiple replicas (e.g. behind a queue), each with its own `Pool` — as a sizing rule of thumb, budget roughly 300-500MB of RSS per warm Chrome instance.

**Operational note on orphaned Chrome processes:** `Pool.Close()` cleanly kills every Chrome instance it launched, but that only runs if your process gets the chance to shut down gracefully. If your process is killed abruptly (`SIGKILL`, an OOM-kill, a hard container stop), the OS-level safeguard (`Pdeathsig`) that's *supposed* to catch this was verified during development to be unreliable for Chrome specifically — it didn't prevent orphaning in testing even via chromedp's own default mechanism, likely due to a combination of [a known Go/Linux limitation](https://go.dev/issue/27505) and Chrome's own internal process handling.

Set `PoolConfig.Supervised: true` to fix this directly rather than relying on operational practice alone:

```go
pool, err := browser.NewPool(context.Background(), browser.PoolConfig{
	Size:       2,
	Supervised: true,
})
```

Each Chrome instance then runs under an intermediary supervisor process (re-exec'd from your own binary — the same "surviving intermediary" pattern `tini`/`dumb-init` use as a container's PID 1), which becomes Chrome's real OS-level parent and blocks on a pipe your original process holds open. If your process dies for any reason, including an unmaskable `SIGKILL`, the OS closes that pipe automatically, and the supervisor kills Chrome (and, transitively, its entire renderer/GPU/zygote process tree) and exits. Verified end-to-end (`browser/supervisor_integration_test.go`): killing the parent process leaves zero surviving processes behind, versus a full orphaned tree without it. On Linux only; requires `os.Executable()` to resolve to a re-runnable copy of your binary (true for normal builds).

If you can't use `Supervised` (e.g. non-Linux), the fallback is still running under an external process supervisor that reaps orphaned children (`tini`/`dumb-init` as your container's PID 1, or your orchestrator's own child-reaping).

### Avoiding bot detection

Plain headless Chrome is fingerprintable — real sites (Cloudflare, DataDome, and similar) can detect and block it. `PoolConfig.Stealth` controls how hard the pool tries to look like a normal browser:

```go
pool, err := browser.NewPool(context.Background(), browser.PoolConfig{
	Size:              2,
	MaxTabsPerBrowser: 4,
	Stealth:           browser.StealthBasic, // or browser.StealthHeadful
})
```

- `browser.StealthOff` (default) — no changes, existing behavior.
- `browser.StealthBasic` — fixes the automation tells verified during development: the `enable-automation` launch flag (and the `navigator.webdriver: true` it causes), a `"HeadlessChrome"` user-agent string, and a WebGL vendor/renderer string that reveals software rendering (a side effect of running with GPU disabled for stability). Stays headless.
- `browser.StealthHeadful` — everything in `StealthBasic`, plus runs a real (non-headless) Chrome inside a virtual X11 display (`Xvfb`) instead of passing `--headless` at all, since headless mode itself has characteristics no amount of flag/JS patching fully hides. **Requires `Xvfb` (and `xauth`) installed and on `PATH`; Linux only.**

Both levels were validated against a real site that outright blocks plain headless Chrome (Unsplash, behind a bot-detection wall) — `StealthBasic` alone was enough to get through in that case.

### Routing through a proxy

`PoolConfig.Proxy` routes every Chrome instance's traffic through an upstream proxy you supply — this wires up *using* your proxy, it doesn't integrate with or recommend any specific provider:

```go
pool, err := browser.NewPool(context.Background(), browser.PoolConfig{
	Proxy: &browser.ProxyConfig{
		Server:   "http://proxy.example.com:8080", // or socks5://...
		Username: "user",                          // leave both empty for an unauthenticated proxy
		Password: "pass",
	},
})
```

Authenticated proxies are handled over CDP (`Fetch.authRequired`/`continueWithAuth`), since Chrome has no way to show its native credential prompt in headless mode. Chrome's proxy is a launch-time setting, not per-tab or per-request — "rotating" proxies means changing `ProxyConfig` on recycled instances (`PoolConfig.MaxUsesPerBrowser`/`MaxBrowserAge`), not switching mid-session.

### Solving CAPTCHAs (Cloudflare Turnstile)

The `captcha` package solves Cloudflare Turnstile challenges via a paid third-party solver, wired into `screenshot.Capture` as an opt-in:

```go
import "github.com/devnyxie/katsuragi/captcha"

result, err := screenshot.Capture(ctx, pool, screenshot.Request{
	URL: "https://example.com",
	CaptchaSolver: &captcha.CapSolver{
		APIKey: "your-capsolver-api-key",
	},
	WaitConfig: browser.WaitUntilNetworkIdle(15 * time.Second),
})
// result.CaptchaSolved is true if a challenge was detected and solved.
```

`captcha.Solver` is a small interface (`Solve(ctx, SolveRequest) (token string, err error)`), so other providers are a config change, not a rewrite. This is a real, per-solve paid dependency with unit-economics impact on a hosted product, and on some sites a genuine terms-of-service consideration — worth weighing per target rather than enabling everywhere by default (`CaptchaSolver` is nil unless you set it).

## HTTP API Server

The `server` package wraps the extraction and screenshot APIs above in a plain `net/http` handler. It's a library, not just a standalone service — import it directly and mount its routes into your own `http.Server`/mux, or let it run its own listener:

```go
import (
	"context"

	"github.com/devnyxie/katsuragi/browser"
	kserver "github.com/devnyxie/katsuragi/server"
)

func main() {
	srv, err := kserver.New(context.Background(), kserver.Config{
		Port:            8080,
		BrowserPoolSize: 2,
		BrowserMaxTabs:  4,
		Stealth:         browser.StealthBasic,
		Supervised:      true,
	})
	if err != nil {
		panic(err)
	}
	defer srv.Close()

	// Either run it directly...
	_ = srv.ListenAndServe(context.Background())

	// ...or mount it into your own mux instead:
	// mux.Handle("/katsuragi/", http.StripPrefix("/katsuragi", srv.Handler()))
}
```

Routes: `GET /healthz`, `GET /v1/extract?url=...&fields=title,description,favicons,links`, `GET /v1/screenshot?url=...&width=&height=&fullPage=&format=png|jpeg&waitUntil=networkidle|selector|fully_loaded&waitSelector=&timeoutMs=`.

`cmd/server` is a ready-to-run binary on top of the same package, configured via environment variables:

```bash
go run ./cmd/server
# PORT=8080 BROWSER_POOL_SIZE=2 BROWSER_MAX_TABS=4 STEALTH_LEVEL=basic SUPERVISED=true go run ./cmd/server
```

| Env var | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | Listen port |
| `BROWSER_POOL_SIZE` | `2` | Warm Chrome instances |
| `BROWSER_MAX_TABS` | `4` | Concurrent tabs per instance |
| `STEALTH_LEVEL` | `basic` | `off`, `basic`, or `headful` |
| `SUPERVISED` | `true` | Orphan-preventing supervisor (see above); Linux only |

# Local Development

## Testing

```bash
go test -v
```

## Integration Testing

The `browser`, `screenshot`, and `server` packages' integration tests drive a real Chrome/Chromium instance and are gated behind the `integration` build tag so the default `go test` run stays fast and dependency-free:

```bash
go test -v -tags=integration ./...
```

The `StealthHeadful` tests additionally require `Xvfb` and `xauth` installed, and are further gated behind Linux via Go's own `linux` build tag (they're skipped entirely when compiling for other platforms, even with `-tags=integration`).

## Code Coverage

```bash
# Generate coverage.out report, generate HTML report from coverage.out, and open the HTML report in the browser:
go test -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html && open coverage.html
```

# License

This project is licensed under the GNU General Public License (GPL). You can find the full text of the license [here](https://www.gnu.org/licenses/gpl-3.0.en.html).
