package api

import "html/template"

// gatePageData/gateAuthPageData feed the two standalone, server-rendered
// pages in gate.go. These are deliberately NOT part of the SvelteKit SPA
// build (web/): they need to render correctly on an arbitrary deployment's
// own domain, independent of and before any app code runs, so they're plain
// html/template with an inline, hand-matched copy of the dashboard's
// "Intertidal" tokens (see web/src/lib/styles.css and DESIGN.md) rather
// than the SPA's bundle. Fonts fall back to the system stack when Figtree /
// JetBrains Mono aren't installed -- nothing is fetched from elsewhere.
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

// sleepPageData feeds the "waking up" page a sleep-enabled deployment's
// first visitor after idle sleep sees -- see Server.renderWakingPage in
// gate.go.
type sleepPageData struct {
	Hostname string
	Name     string
}

// gateStyles is shared by all three pages: the dashboard's sign-in card
// (tide banner on top, mangrove mark, leaf-green primary button), light by
// default and its dark appearance under prefers-color-scheme -- there's no
// per-visitor appearance setting to read on someone else's domain.
const gateStyles = `<style>
:root {
  --bg: #f2f1ec;
  --bg-card: #ffffff;
  --bg-input: #fbfbf8;
  --border: #e4e2da;
  --border-field: #e1dfd6;
  --border-strong: #c9c6ba;
  --text: #1b2420;
  --text-strong: #44504a;
  --text-dim: #6b746e;
  --text-faint: #7c847e;
  --leaf: #1f5c47;
  --leaf-bright: #174a39;
  --leaf-wash: #eaf3ee;
  --red-bright: #a23b2a;
  --red-ink: #8e2f1f;
  --red-wash: #f8e1db;
  --red-line: #ebc9c0;
  --sans: "Figtree", -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, sans-serif;
  --mono: "JetBrains Mono", "SF Mono", ui-monospace, Menlo, Consolas, monospace;
  color-scheme: light;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0e1512;
    --bg-card: #151f1b;
    --bg-input: #0f1814;
    --border: #263530;
    --border-field: #2a3b34;
    --border-strong: #3a5248;
    --text: #e7eee9;
    --text-strong: #b6c6bd;
    --text-dim: #8fa298;
    --text-faint: #788b81;
    --leaf: #2a7a5f;
    --leaf-bright: #33906f;
    --leaf-wash: #1c3731;
    --red-bright: #f29a86;
    --red-ink: #f29a86;
    --red-wash: #352b23;
    --red-line: #5c3b33;
    color-scheme: dark;
  }
}
* { box-sizing: border-box; }
body {
  margin: 0;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  background: var(--bg);
  color: var(--text);
  font-family: var(--sans);
  font-size: 14px;
  line-height: 1.45;
  -webkit-font-smoothing: antialiased;
}
.mono { font-family: var(--mono); font-size: 0.93em; }
.card {
  width: 420px;
  max-width: 100%;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 20px;
  box-shadow: 0 1px 2px rgba(27, 36, 32, 0.04);
  overflow: hidden;
}
.banner { height: 120px; }
.tide-banner { display: block; width: 100%; height: 100%; }
.tide-banner.night { display: none; }
@media (prefers-color-scheme: dark) {
  .tide-banner.day { display: none; }
  .tide-banner.night { display: block; }
}
.body { padding: 24px 26px 26px; }
.mark { width: 34px; height: 34px; color: var(--leaf); margin-bottom: 12px; display: block; }
h1 { font-size: 24px; font-weight: 700; letter-spacing: -0.02em; line-height: 1.2; margin: 0 0 6px; }
p.subtitle { color: var(--text-dim); margin: 0 0 20px; font-size: 14px; }
p.subtitle .host { color: var(--text); }
.field { margin-bottom: 14px; }
label { display: block; font-size: 12px; font-weight: 600; color: var(--text-strong); margin-bottom: 6px; }
input[type="email"], input[type="password"] {
  width: 100%;
  padding: 9px 12px;
  border-radius: 10px;
  border: 1px solid var(--border-field);
  background: var(--bg-input);
  color: var(--text);
  font: inherit;
  outline: none;
  transition: border-color 0.15s, box-shadow 0.15s;
}
input:focus { border-color: var(--leaf); box-shadow: 0 0 0 3px var(--leaf-wash); }
.btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  padding: 9px 14px;
  border-radius: 10px;
  border: 1px solid var(--border-field);
  background: var(--bg-card);
  color: var(--text);
  font: 600 13px var(--sans);
  cursor: pointer;
  text-decoration: none;
  transition: border-color 0.15s, background 0.15s, transform 0.1s;
}
.btn:hover { border-color: var(--border-strong); }
.btn:active { transform: scale(0.98); }
.btn-primary { background: var(--leaf); border-color: var(--leaf); color: #ffffff; }
.btn-primary:hover { background: var(--leaf-bright); border-color: var(--leaf-bright); }
.divider {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 18px 0;
  color: var(--text-faint);
  font-size: 12px;
}
.divider::before, .divider::after { content: ""; flex: 1; height: 1px; background: var(--border); }
.error-banner {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  background: var(--red-wash);
  border: 1px solid var(--red-line);
  color: var(--red-ink);
  padding: 10px 14px;
  border-radius: 14px;
  margin-bottom: 14px;
  font-size: 13px;
}
.error-banner svg { width: 15px; height: 15px; margin-top: 2px; flex-shrink: 0; color: var(--red-bright); }
.note { margin: 16px 0 0; font-size: 12px; color: var(--text-dim); }
</style>`

// mangroveMarkSVG is the dashboard's brand mark (web/src/lib/icons.ts's
// "mangrove"), drawn the way Icon.svelte draws every icon.
const mangroveMarkSVG = `<svg class="mark" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 21c0-7 3.5-10 8-10s8 3 8 10"/><path d="M8.5 21c0-4.5 1.4-7 3.5-7s3.5 2.5 3.5 7"/><path d="M12 11V3"/></svg>`

const warnIconSVG = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 4 3 20h18z"/><path d="M12 10v4M12 17h.01"/></svg>`

// sleepTpl is the "waking up" page a sleep-enabled deployment's visitor
// sees while its containers restart -- see Server.renderWakingPage. The
// banner is the "turning" (in-progress) scene, and a meta-refresh instead of
// JS keeps gate.go's no-JS-dependency convention.
var sleepTpl = template.Must(template.New("sleep").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="3">
<title>Waking up</title>
` + gateStyles + `
</head>
<body>
<main class="card">
  ` + gateBanner(bannerMidDay, bannerMidNight) + `
  <div class="body">
    ` + mangroveMarkSVG + `
    <h1>Waking up{{if .Name}} {{.Name}}{{end}}</h1>
    <p class="subtitle">This app went to sleep after a quiet spell and is starting back up. This page refreshes on its own; it usually takes a few seconds.</p>
  </div>
</main>
</body>
</html>
`))

var gateTpl = template.Must(template.New("gate").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Protected</title>
` + gateStyles + `
</head>
<body>
<main class="card">
  ` + gateBanner(bannerHighDay, bannerHighNight) + `
  <div class="body">
    ` + mangroveMarkSVG + `
    <h1>This deployment is protected</h1>
    <p class="subtitle">Sign in with a Mangrove account, or enter the password, to view <span class="host mono">{{.Hostname}}</span>.</p>
    {{if .ErrMsg}}<div class="error-banner" role="alert">` + warnIconSVG + `<span>{{.ErrMsg}}</span></div>{{end}}
    {{if .AccountURL}}
    <a class="btn btn-primary" href="{{.AccountURL}}">Continue with Mangrove account</a>
    <div class="divider">or</div>
    {{end}}
    <form method="POST" action="/__mangrove_gate__/password">
      <input type="hidden" name="returnTo" value="{{.ReturnTo}}">
      <div class="field">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" autocomplete="current-password" required autofocus>
      </div>
      <button type="submit" class="btn{{if not .AccountURL}} btn-primary{{end}}">Unlock</button>
    </form>
  </div>
</main>
</body>
</html>
`))

var gateAuthTpl = template.Must(template.New("gateAuth").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in</title>
` + gateStyles + `
</head>
<body>
<main class="card">
  ` + gateBanner(bannerHighDay, bannerHighNight) + `
  <div class="body">
    ` + mangroveMarkSVG + `
    {{if .IsError}}
    <h1>Link expired</h1>
    <p class="subtitle">{{.ErrMsg}}</p>
    {{else}}
    <h1>Sign in to continue</h1>
    <p class="subtitle">Sign in with your Mangrove account to open the protected deployment you came from.</p>
    {{if .ErrMsg}}<div class="error-banner" role="alert">` + warnIconSVG + `<span>{{.ErrMsg}}</span></div>{{end}}
    <form method="POST" action="/gate-auth">
      <input type="hidden" name="token" value="{{.Token}}">
      <div class="field">
        <label for="email">Email</label>
        <input type="email" id="email" name="email" autocomplete="username" required autofocus>
      </div>
      <div class="field">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" autocomplete="current-password" required>
      </div>
      <button type="submit" class="btn btn-primary">Sign in</button>
    </form>
    {{end}}
  </div>
</main>
</body>
</html>
`))
