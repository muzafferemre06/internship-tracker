FROM golang:1.26.5-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/backup ./cmd/backup \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/restorecheck ./cmd/restorecheck \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dbinspect ./cmd/dbinspect

FROM alpine:3.24.1

RUN apk add --no-cache tzdata wget && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY --from=builder /out/backup /app/backup
COPY --from=builder /out/restorecheck /app/restorecheck
COPY --from=builder /out/dbinspect /app/dbinspect
COPY migrations /app/migrations
RUN mkdir -p /app/data /app/backups && chown -R app:app /app

USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
