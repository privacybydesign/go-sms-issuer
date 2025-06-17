FROM node:23 AS frontend-build
WORKDIR /app/frontend
COPY frontend .
RUN npm install
RUN npm run build

# -----------------------------------------------------

FROM golang:1.23 AS backend-build
WORKDIR /app/backend
COPY backend .
RUN go mod download

# compile with static linking
RUN CGO_ENABLED=0 go build -o server 

# -----------------------------------------------------

FROM alpine:latest

COPY --from=backend-build /app/backend/server /app/backend/server
COPY --from=frontend-build /app/frontend /app/frontend/build

WORKDIR /app/backend
EXPOSE 8080
CMD ["/bin/sh", "-c", "cp ./server --config /secrets/config.json"]
