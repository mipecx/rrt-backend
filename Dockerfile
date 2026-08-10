FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o rrt_app ./cmd/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/rrt_app .
COPY --from=builder /app/migrations ./internal/migrations

EXPOSE 8080
CMD ["./rrt_app"]
