# check=skip=SecretsUsedInArgOrEnv
# TURNSTILE_SITE_KEY is the *public* Cloudflare Turnstile site key — it is
# bundled into the client JS by Vite (VITE_* prefix) and rendered into the
# served HTML, so passing it via ARG is correct. The matching secret key
# lives only in backend config.

FROM node:25@sha256:78839ac448c23517f8eab2e8f7943d9b4f73979eb7f8bed2c73dbf72ff869e7b AS frontend-build
WORKDIR /app/frontend
COPY frontend .

# Accept build-time argument for the env var
ARG TURNSTILE_SITE_KEY

ENV VITE_TURNSTILE_SITE_KEY=$TURNSTILE_SITE_KEY

RUN npm install
RUN npm run build

# -----------------------------------------------------

FROM golang:1.27.2@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8 AS backend-build
WORKDIR /app/backend
COPY backend .
RUN go mod download

# compile with static linking
RUN CGO_ENABLED=0 go build -o ./server

# -----------------------------------------------------

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS runtime
WORKDIR /app/backend

COPY --from=backend-build /app/backend/server /app/backend
COPY --from=frontend-build /app/frontend/build/ /app/frontend/build

# Declared explicitly to satisfy static scanners; the :nonroot tag already
# defaults to this same user (UID 65532).
USER nonroot:nonroot

EXPOSE 8080
ENTRYPOINT [ "/app/backend/server", "--config", "/secrets/config.json" ]
