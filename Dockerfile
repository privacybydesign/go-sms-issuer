# check=skip=SecretsUsedInArgOrEnv
# TURNSTILE_SITE_KEY is the *public* Cloudflare Turnstile site key — it is
# bundled into the client JS by Vite (VITE_* prefix) and rendered into the
# served HTML, so passing it via ARG is correct. The matching secret key
# lives only in backend config.

FROM node:24@sha256:6449e19d8184d84062525b6a7524def03146c2cb04a7f08b076680d0e8e4240a AS frontend-build
WORKDIR /app/frontend
COPY frontend .

# Accept build-time argument for the env var
ARG TURNSTILE_SITE_KEY

ENV VITE_TURNSTILE_SITE_KEY=$TURNSTILE_SITE_KEY

RUN npm install
RUN npm run build

# -----------------------------------------------------

FROM golang:1.26.10@sha256:672c944cfd07599b5f1bb179869270cdc94c6254e41db4d431259693cb286b82 AS backend-build
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
