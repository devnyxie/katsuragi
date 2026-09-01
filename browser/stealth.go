package browser

import (
	"context"
	goruntime "runtime"
	"strings"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// StealthLevel controls how hard a Pool tries to avoid looking automated.
type StealthLevel int

const (
	// StealthOff launches Chrome with chromedp's standard defaults —
	// no stealth behavior. The default.
	StealthOff StealthLevel = iota
	// StealthBasic applies the launch-flag fix and fingerprint patches
	// from this file while staying in headless mode.
	StealthBasic
	// StealthHeadful applies everything in StealthBasic, plus runs a
	// real (non-headless) Chrome inside a virtual X11 display (Xvfb)
	// rather than passing --headless at all — headless mode itself has
	// detectable characteristics no amount of flag/JS patching fully
	// hides. Requires Xvfb on PATH; Linux only.
	StealthHeadful
)

// chPlatform maps the host OS to the value real Chrome reports in the
// Client Hints Sec-CH-UA-Platform header / navigator.userAgentData.platform,
// so the UA override stays internally consistent with the actual host
// rather than claiming a platform that Client Hints would contradict.
func chPlatform() string {
	switch goruntime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	default:
		return "Linux"
	}
}

// stealthJS is injected into every document (via
// Page.addScriptToEvaluateOnNewDocument, so it runs before any page script)
// on stealth-mode tabs. It patches WebGL's UNMASKED_VENDOR_WEBGL/
// UNMASKED_RENDERER_WEBGL, which report a software rasterizer ("Google
// Inc." / "... SwiftShader ...") because the pool runs with GPU disabled
// for stability (see Pool doc comment) — a visible tell distinguishing
// this from a GPU-equipped user. This patch reports a plausible real GPU
// string instead.
//
// An earlier version of this patch also overrode
// navigator.permissions.query('notifications') to match
// Notification.permission, based on a "denied" reading that turned out to
// be an artifact of testing against about:blank rather than a real page —
// on an actual navigation, unpatched Chrome (headless or not) already
// reports the spec-correct pair (Notification.permission: "default",
// permissions.query: "prompt"; they intentionally use different
// vocabularies for the same "not yet decided" state). Patching it to make
// them match would have produced a non-spec value ("default" is not a
// valid PermissionStatus.state), a worse tell than doing nothing — so
// that patch was removed once verified against a real page.
//
// navigator.webdriver is deliberately NOT patched here either: it's fixed
// at the browser level via the disable-blink-features=AutomationControlled
// launch flag (see stealthExecOptions), which is more robust than a JS
// override — a property-descriptor override is itself detectable by
// inspecting Object.getOwnPropertyDescriptor, whereas the native flag
// leaves no trace.
const stealthJS = `(() => {
  try {
    var proto = WebGLRenderingContext.prototype;
    var orig = proto.getParameter;
    proto.getParameter = function (parameter) {
      if (parameter === 37445) return 'Intel Inc.';
      if (parameter === 37446) return 'Intel Iris OpenGL Engine';
      return orig.call(this, parameter);
    };
    if (window.WebGL2RenderingContext) {
      var proto2 = WebGL2RenderingContext.prototype;
      var orig2 = proto2.getParameter;
      proto2.getParameter = function (parameter) {
        if (parameter === 37445) return 'Intel Inc.';
        if (parameter === 37446) return 'Intel Iris OpenGL Engine';
        return orig2.call(this, parameter);
      };
    }
  } catch (e) {}
})();`

// stealthExecOptions returns launch flags that avoid the automation tells
// verified during development: chromedp.DefaultExecAllocatorOptions
// includes Flag("enable-automation", true), which is what flips
// navigator.webdriver and related internals. This builds an equivalent
// flag set with that omitted, plus disable-blink-features=
// AutomationControlled, which is the flag that actually fixes
// navigator.webdriver (verified empirically — no JS override needed).
//
// headless controls whether the --headless flag is included at all. It is
// false for StealthHeadful, where the caller runs Chrome inside a virtual
// display (Xvfb) instead — see xvfb_linux.go.
func stealthExecOptions(headless bool) []chromedp.ExecAllocatorOption {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-features", "site-per-process,Translate,BlinkGenPropertyTrees"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
		chromedp.WindowSize(1920, 1080),
	}
	if headless {
		opts = append(opts, chromedp.Headless)
	}
	return opts
}

// fetchAndStripUserAgent reads the browser's real user-agent string via CDP
// and strips "Headless" from it. The real string (not a hardcoded one) is
// used deliberately: hardcoding risks drifting out of sync with the actual
// Chrome version, which would itself be an inconsistency (e.g. between the
// UA string and navigator.userAgentData, which Chrome derives from its real
// binary version regardless of any override).
func fetchAndStripUserAgent(ctx context.Context) (string, error) {
	var realUA string
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, _, ua, _, err := browser.GetVersion().Do(ctx)
		realUA = ua
		return err
	}))
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(realUA, "HeadlessChrome", "Chrome"), nil
}

// applyStealthToTab overrides the tab's user-agent (both the classic string
// and the Client Hints metadata, kept consistent with each other and with
// the real platform to avoid a worse tell than doing nothing) and injects
// stealthJS so it runs before any page script on every subsequent
// navigation in this tab.
func applyStealthToTab(tabCtx context.Context, userAgent string) error {
	return chromedp.Run(tabCtx,
		emulation.SetUserAgentOverride(userAgent).WithUserAgentMetadata(&emulation.UserAgentMetadata{
			Platform:     chPlatform(),
			Architecture: "x86",
			Mobile:       false,
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(stealthJS).Do(ctx)
			return err
		}),
	)
}
