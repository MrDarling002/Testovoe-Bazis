FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api ./cmd/api

FROM alpine:3.20

RUN adduser -D -u 10001 app

COPY --from=builder /bin/api /app/api

USER app

EXPOSE 8080

ENTRYPOINT ["/app/api"]