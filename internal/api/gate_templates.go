package api

import "html/template"

// gatePageData/gateAuthPageData feed the two standalone, server-rendered
// pages in gate.go. These are deliberately NOT part of the React SPA build
// (web/): they need to render correctly on an arbitrary deployment's own
// domain, independent of and before any app code runs, so they're plain
// html/template with an inline, hand-matched subset of the dashboard's
// "Field Station" palette (see web/src/styles.css) rather than the
// self-hosted fonts/bundle the SPA uses.
type gatePageData struct {
	Hostname   string
	ReturnTo   string
	ErrMsg     string
	AccountURL string // empty omits the "continue with your Mangrove account" button
}

type gateAuthPageData struct {
	Token   string
	ErrMsg  string
	IsError bool
}

// gateStyles is shared between both pages: same dark card, same brass
// accent, same fallback font stacks (no external font loading -- this page
// is served standalone, outside the SPA's self-hosted @fontsource bundle).
const gateStyles = `<style>
:root {
  --bg: #14100a;
  --bg-card: #201911;
  --border: #362a1a;
  --border-hover: #4d3c25;
  --text: #ece3d2;
  --text-dim: #a99b83;
  --text-faint: #6f6250;
  --brass: #caa057;
  --brass-bright: #e2bd76;
  --brass-dim: #7a6236;
  --brass-wash: rgba(202, 160, 87, 0.14);
  --red: #cc5c53;
  --red-wash: rgba(204, 92, 83, 0.14);
  --radius-md: 8px;
  --radius-lg: 12px;
  color-scheme: dark;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
  background: var(--bg);
  color: var(--text);
  font-family: "Space Grotesk", -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
}
.mono {
  font-family: "IBM Plex Mono", "SF Mono", ui-monospace, Menlo, Consolas, monospace;
}
.card {
  width: 100%;
  max-width: 420px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  box-shadow: 0 28px 72px rgba(0, 0, 0, 0.55);
  padding: 2rem;
  text-align: center;
}
.icon-badge {
  width: 52px;
  height: 52px;
  margin: 0 auto 1.25rem;
  border-radius: 999px;
  background: var(--brass-wash);
  border: 1px solid var(--border-hover);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--brass-bright);
}
h1 {
  margin: 0 0 0.5rem;
  font-size: 1.4rem;
  font-weight: 600;
  color: var(--text);
}
p.subtext {
  margin: 0 0 1.5rem;
  font-size: 0.9rem;
  line-height: 1.5;
  color: var(--text-dim);
}
p.subtext .host { color: var(--text); }
.btn {
  display: block;
  width: 100%;
  padding: 0.7rem 1rem;
  border-radius: var(--radius-md);
  border: none;
  font-size: 0.9rem;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  text-decoration: none;
  box-sizing: border-box;
}
.btn-primary {
  background: var(--brass);
  color: #14100a;
}
.btn-primary:hover { background: var(--brass-bright); }
.btn-secondary {
  background: transparent;
  color: var(--text);
  border: 1px solid var(--border-hover);
}
.btn-secondary:hover { border-color: var(--brass); }
.divider {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: 1.25rem 0;
  color: var(--text-faint);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.divider::before, .divider::after {
  content: "";
  flex: 1;
  height: 1px;
  background: var(--border);
}
form { text-align: left; }
label {
  display: block;
  font-size: 0.8rem;
  color: var(--text-dim);
  margin-bottom: 0.4rem;
}
input[type="email"], input[type="password"] {
  width: 100%;
  padding: 0.6rem 0.75rem;
  margin-bottom: 1rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  color: var(--text);
  font-family: inherit;
}
input[type="password"] { font-family: "IBM Plex Mono", "SF Mono", ui-monospace, Menlo, Consolas, monospace; }
input:focus { outline: none; border-color: var(--brass); }
.error-banner {
  background: var(--red-wash);
  border: 1px solid var(--red);
  color: var(--red);
  border-radius: var(--radius-md);
  padding: 0.6rem 0.75rem;
  font-size: 0.85rem;
  margin-bottom: 1rem;
  text-align: left;
}
</style>`

const lockIconSVG = `<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="11" width="16" height="9" rx="2"></rect><path d="M8 11V7a4 4 0 0 1 8 0v4"></path></svg>`

var gateTpl = template.Must(template.New("gate").Parse(gateStyles + `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Protected</title>
</head>
<body>
<div class="card">
  <div class="icon-badge">` + lockIconSVG + `</div>
  <h1>This deployment is protected</h1>
  <p class="subtext">Sign in with a Mangrove account, or enter the password, to view <span class="host mono">{{.Hostname}}</span>.</p>
  {{if .ErrMsg}}<div class="error-banner">{{.ErrMsg}}</div>{{end}}
  {{if .AccountURL}}
  <a class="btn btn-primary" href="{{.AccountURL}}">Continue with Mangrove account</a>
  <div class="divider">or</div>
  {{end}}
  <form method="POST" action="/__mangrove_gate__/password">
    <input type="hidden" name="returnTo" value="{{.ReturnTo}}">
    <label for="password">Password</label>
    <input type="password" id="password" name="password" required autofocus>
    <button type="submit" class="btn {{if .AccountURL}}btn-secondary{{else}}btn-primary{{end}}">Unlock</button>
  </form>
</div>
</body>
</html>
`))

var gateAuthTpl = template.Must(template.New("gateAuth").Parse(gateStyles + `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in</title>
</head>
<body>
<div class="card">
  <div class="icon-badge">` + lockIconSVG + `</div>
  {{if .IsError}}
  <h1>Link expired</h1>
  <p class="subtext">{{.ErrMsg}}</p>
  {{else}}
  <h1>Sign in to continue</h1>
  <p class="subtext">Signing in with your Mangrove account grants access to the protected deployment you came from.</p>
  {{if .ErrMsg}}<div class="error-banner">{{.ErrMsg}}</div>{{end}}
  <form method="POST" action="/gate-auth">
    <input type="hidden" name="token" value="{{.Token}}">
    <label for="email">Email</label>
    <input type="email" id="email" name="email" required autofocus>
    <label for="password">Password</label>
    <input type="password" id="password" name="password" required>
    <button type="submit" class="btn btn-primary">Sign in</button>
  </form>
  {{end}}
</div>
</body>
</html>
`))
