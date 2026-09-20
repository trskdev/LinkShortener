FROM golang:1.26-alpine AS builder
WORKDIR /app
RUN apk add --no-cache curl
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-amd64.tar.gz | tar xvz
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/link_shortener/main.go


FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
COPY --from=builder /app/migrate /usr/local/bin/migrate
COPY --from=builder /app/migrations ./migrations
EXPOSE 8080
CMD ["./main"]