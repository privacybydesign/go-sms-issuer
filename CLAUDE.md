# go-sms-issuer — repo notes

Go backend (`backend/`) plus a React frontend (`frontend/`). The backend issues
SMS one-time codes and exchanges them for an IRMA/Yivi issuance JWT.

## Build & test

- Backend: `cd backend && go build ./... && go test ./...`. Go version is set in
  `backend/go.mod` (currently 1.26).
- Tests spin up a real HTTP server on `127.0.0.1:8081` and use in-memory
  storage; no Redis is needed for `go test`.

## Gotchas

- `github.com/altcha-org/altcha-lib-go/v2` is **untagged** upstream: it resolves
  to a pseudo-version (`v2.0.0-<date>-<commit>`), which is pinned in `go.mod`. Do
  not float it to `@latest`. The Go and Dart ALTCHA bindings only interoperate on
  **v2** with the algorithm pinned to `PBKDF2/SHA-256` (root/v1 is not wire
  compatible). See `backend/altcha/`.
- Two send paths share `sendSms`: `/send` (public web, Turnstile captcha) and
  `/api/embedded/send` (Yivi app, optional ALTCHA proof of work). Keep abuse
  controls on the embedded path in `handleEmbeddedIssuanceSendSms`.
