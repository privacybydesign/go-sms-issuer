# check=skip=SecretsUsedInArgOrEnv
# TURNSTILE_SITE_KEY is the *public* Cloudflare Turnstile site key — it is
# bundled into the client JS by Vite (VITE_* prefix) and rendered into the
# served HTML, so passing it via ARG is correct. The matching secret key
# lives only in backend config.

FROM node:23 AS frontend-build
WORKDIR /app/frontend
COPY frontend .

# Accept build-time argument for the env var
ARG TURNSTILE_SITE_KEY

ENV VITE_TURNSTILE_SITE_KEY=$TURNSTILE_SITE_KEY

RUN npm install
RUN npm run build

# -----------------------------------------------------

FROM golang:1.26 AS backend-build
WORKDIR /app/backend
COPY backend .
RUN go mod download

# compile with static linking
RUN CGO_ENABLED=0 go build -o ./server

# -----------------------------------------------------

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app/backend

COPY --from=backend-build /app/backend/server /app/backend
COPY --from=frontend-build /app/frontend/build/ /app/frontend/build

EXPOSE 8080
ENTRYPOINT [ "/app/backend/server", "--config", "/secrets/config.json" ]
