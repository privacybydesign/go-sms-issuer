FROM node:23 AS frontend-build

WORKDIR /app/frontend

COPY frontend .

RUN yarn install
RUN ./build.sh en

# -----------------------------------------------------

FROM golang:1.23 AS backend-build

WORKDIR /app/backend

COPY backend .
RUN go mod download

RUN go build -o server

# -----------------------------------------------------

FROM alpine:latest

WORKDIR /app

COPY --from=backend-build /app/backend/server ./backend
COPY --from=frontend-build /app/frontend ./frontend

EXPOSE 8080

CMD ["./backend/server"]
