// Package captcha provides a provider-agnostic interface for solving
// CAPTCHA challenges — Cloudflare Turnstile initially — encountered while
// capturing a page, plus one concrete implementation (CapSolver).
//
// Solving a CAPTCHA this way means paying a third-party service to defeat
// a site's bot mitigation on your behalf. That's a real per-solve cost
// with unit-economics implications for a hosted product, and on some
// sites a genuine terms-of-service consideration worth weighing per
// target — this package doesn't take a position on either, it just
// provides the mechanism.
package captcha

import "context"

// ChallengeType identifies which CAPTCHA product a SolveRequest is for.
type ChallengeType string

const (
	Turnstile ChallengeType = "turnstile"
)

// SolveRequest describes one CAPTCHA challenge to solve.
type SolveRequest struct {
	Type ChallengeType
	// SiteKey is the site's public widget key (e.g. the Turnstile
	// data-sitekey attribute).
	SiteKey string
	// PageURL is the URL of the page presenting the challenge.
	PageURL string
}

// Solver solves a CAPTCHA challenge out-of-band and returns the resulting
// response token to inject into the page.
type Solver interface {
	Solve(ctx context.Context, req SolveRequest) (token string, err error)
}
