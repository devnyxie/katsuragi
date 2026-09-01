package screenshot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/chromedp"

	"github.com/devnyxie/katsuragi/captcha"
)

// detectTurnstile checks the current page for a Cloudflare Turnstile
// widget and returns its sitekey if present.
func detectTurnstile(tabCtx context.Context) (siteKey string, found bool, err error) {
	const script = `(() => {
		const el = document.querySelector('.cf-turnstile[data-sitekey], [data-sitekey]');
		return el ? el.getAttribute('data-sitekey') : '';
	})()`
	var key string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(script, &key)); err != nil {
		return "", false, err
	}
	return key, key != "", nil
}

// solveTurnstile solves the detected challenge via solver and injects the
// resulting token into the page, then triggers whatever mechanism the page
// uses to continue: the widget's data-callback if one is wired up, and/or
// submitting the enclosing form (Cloudflare's own challenge pages are
// built to auto-continue once the response field is populated, whether by
// the real widget or an injected token — this is best-effort, since the
// exact continuation mechanism varies by site).
func solveTurnstile(ctx context.Context, tabCtx context.Context, solver captcha.Solver, pageURL, siteKey string) error {
	token, err := solver.Solve(ctx, captcha.SolveRequest{
		Type:    captcha.Turnstile,
		SiteKey: siteKey,
		PageURL: pageURL,
	})
	if err != nil {
		return fmt.Errorf("solve failed: %w", err)
	}

	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return err
	}

	injectScript := fmt.Sprintf(`(() => {
		const token = %s;
		const el = document.querySelector('.cf-turnstile[data-sitekey], [data-sitekey]');
		let input = document.querySelector('[name="cf-turnstile-response"]');
		if (!input) {
			input = document.createElement('input');
			input.setAttribute('name', 'cf-turnstile-response');
			input.style.display = 'none';
			(el && el.parentNode ? el.parentNode : document.body).appendChild(input);
		}
		input.value = token;
		input.dispatchEvent(new Event('input', { bubbles: true }));
		input.dispatchEvent(new Event('change', { bubbles: true }));

		const callbackName = el && el.getAttribute('data-callback');
		if (callbackName && typeof window[callbackName] === 'function') {
			window[callbackName](token);
		}

		const form = input.closest('form');
		if (form) {
			if (typeof form.requestSubmit === 'function') {
				form.requestSubmit();
			} else {
				form.submit();
			}
		}
	})()`, tokenJSON)

	return chromedp.Run(tabCtx, chromedp.Evaluate(injectScript, nil))
}
