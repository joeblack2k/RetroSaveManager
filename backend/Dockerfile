# syntax=docker/dockerfile:1.7

FROM golang:1.24-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/retrosave-api ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/retrosave-healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12
WORKDIR /app

COPY --from=build /out/retrosave-api /usr/local/bin/retrosave-api
COPY --from=build /out/retrosave-healthcheck /usr/local/bin/retrosave-healthcheck

ENV PORT=3001
ENV AUTH_MODE=disabled
ENV SAVE_ROOT=/var/lib/retrosavemanager/saves
ENV STATE_ROOT=/var/lib/retrosavemanager/state

EXPOSE 3001

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD ["/usr/local/bin/retrosave-healthcheck"]

ENTRYPOINT ["/usr/local/bin/retrosave-api"]
