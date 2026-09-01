package browser

import (
	"context"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/chromedp"
)

// ProxyConfig configures an upstream proxy every Chrome instance in the
// pool routes its traffic through. This wires up USE of a proxy the
// caller already has — it does not integrate with or recommend any
// specific proxy provider.
type ProxyConfig struct {
	// Server is passed directly to Chrome's --proxy-server flag, e.g.
	// "http://host:port" or "socks5://host:port".
	Server string
	// Username and Password authenticate against Server, if it requires
	// credentials. Leave both empty for an unauthenticated proxy.
	Username string
	Password string
}

func (p *ProxyConfig) authenticated() bool {
	return p != nil && (p.Username != "" || p.Password != "")
}

// enableProxyAuth registers a listener that answers the upstream proxy's
// auth challenge (Fetch.authRequired) with the configured credentials, and
// otherwise lets every request through unmodified.
//
// This is necessary because Chrome has no way to show its native
// username/password prompt in headless mode — without this, every request
// through an authenticated proxy fails outright. Enabling the Fetch domain
// with no URL pattern filter pauses *all* requests (per the CDP spec) until
// explicitly resumed, so the listener also resumes every non-auth request
// via Fetch.continueRequest; we're not modifying request content, just
// intercepting the one event we need.
//
// Chrome's proxy is a process-launch-time setting via --proxy-server, not
// per-tab — so this is applied once per instance's tabs, not tuned
// per-request. "Rotating" proxies means cycling ProxyConfig across
// recycled instances (see PoolConfig.MaxUsesPerBrowser/MaxBrowserAge).
func enableProxyAuth(tabCtx context.Context, proxy ProxyConfig) error {
	chromedp.ListenTarget(tabCtx, func(ev any) {
		switch e := ev.(type) {
		case *fetch.EventAuthRequired:
			go func() {
				_ = chromedp.Run(tabCtx, fetch.ContinueWithAuth(e.RequestID, &fetch.AuthChallengeResponse{
					Response: fetch.AuthChallengeResponseResponseProvideCredentials,
					Username: proxy.Username,
					Password: proxy.Password,
				}))
			}()
		case *fetch.EventRequestPaused:
			go func() {
				_ = chromedp.Run(tabCtx, fetch.ContinueRequest(e.RequestID))
			}()
		}
	})

	return chromedp.Run(tabCtx, fetch.Enable().WithHandleAuthRequests(true))
}
