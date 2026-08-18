# Auth Bridge

One static file (`index.html`), one job: LinkedIn requires an absolute
`https://` redirect URL and won't accept a custom URL scheme directly, so
this page is what LinkedIn actually redirects to — it immediately forwards
whatever LinkedIn sent (`code`/`state`, or an `error`) to the app's real
destination, the `professionalconnections://auth/linkedin/callback` custom
scheme the app is listening for (`frontend/PLAN.md` Step 2). It never talks
to the backend and never sees the client secret — it's pure hand-off.

Branded to match the app (dark, glassmorphism, same palette as
`frontend/lib/core/theme/app_palette.dart`) rather than a bare blank
redirect, since this is the first thing a user sees right after granting
LinkedIn access — a branded "Finishing sign-in…" moment reads as trustworthy
continuity; an unbranded flash reads as a broken handoff, which matters more
than usual for an app whose whole pitch is "we take your safety seriously."

Includes a fallback: most browsers honor the automatic JS redirect to a
custom scheme, but some mobile browsers block script-triggered redirects to
non-http(s) schemes and only allow it from a real tap. A "Open the app"
button appears after ~1.2s in case the automatic redirect didn't fire.

## Deploying it

Doesn't need to live behind the Go backend or Cloud Run — it's a static
file with zero server logic, so the simplest host wins. **Recommended:
Firebase Hosting** — free tier covers this easily, stays inside the
existing GCP project (consistent with the GCP-first stance already decided
for the backend), and setup is three commands:

```bash
npm install -g firebase-tools
firebase login
cd auth-bridge
firebase init hosting   # point the public directory at "." (this folder)
firebase deploy
```

That gives a stable `https://<project-id>.web.app/` URL (or a custom domain
if one's attached later) — **that URL is what gets registered as the
LinkedIn app's Redirect URL**, and what `LINKEDIN_REDIRECT_URI` /
`frontend/PLAN.md`'s `signInWithLinkedIn()` implementation both need to use
as `redirect_uri`. It doesn't change with local vs. production backend
environments — this page never touches the backend, so one deployed bridge
URL serves both.

Any other static host (Cloudflare Pages, GitHub Pages, a Cloud Storage
static-website bucket) works exactly as well; Firebase's just the path of
least friction given the existing GCP account.
