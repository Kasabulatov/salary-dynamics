# --- Dev stage: source is volume-mounted, run with `go run .` ---
FROM golang:1.26-alpine AS dev
RUN apk add --no-cache git curl
WORKDIR /app

# --- Build stage: compile a static binary ---
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server .

# --- Prod stage: minimal runtime image (deploy this to Render/Fly) ---
FROM alpine:3.21 AS prod
RUN apk add --no-cache ca-certificates
COPY --from=build /server /server
COPY --from=build /app/migrations /migrations
ENV MIGRATIONS_DIR=/migrations
EXPOSE 8080
CMD ["/server"]
