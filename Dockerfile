FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/client ./cmd/client

FROM alpine:3.20
WORKDIR /app

RUN adduser -D -u 10001 appuser
COPY --from=builder /out/server /app/server
COPY --from=builder /out/client /app/client
COPY .env.server /app/.env.server
COPY .env.client /app/.env.client
COPY cmd /app/cmd

USER appuser
CMD ["/app/server"]
