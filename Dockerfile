FROM node:23 AS frontend-build
WORKDIR /app/frontend
COPY frontend .

RUN npm install
RUN npm run build

# -----------------------------------------------------

FROM golang:1.24 AS backend-build
WORKDIR /app/backend
COPY backend .
RUN go mod download

# compile with static linking
RUN CGO_ENABLED=0 go build -o ./server

# -----------------------------------------------------

FROM golang:1.24 AS runtime
WORKDIR /app/backend

COPY --from=backend-build /app/backend/server /app/backend
COPY --from=frontend-build /app/frontend/build/ /app/frontend/build

EXPOSE 8080
ENTRYPOINT [ "/app/backend/server", "--config", "/secrets/config.json" ]
