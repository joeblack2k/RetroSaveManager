# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS frontend-build
WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
ARG VITE_AUTH_REQUIRED=false
ENV VITE_AUTH_REQUIRED=${VITE_AUTH_REQUIRED}
RUN npm run build

FROM golang:1.24-alpine AS backend-build
WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/retrosave-api ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/retrosave-healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12
WORKDIR /app

COPY --from=backend-build /out/retrosave-api /usr/local/bin/retrosave-api
COPY --from=backend-build /out/retrosave-healthcheck /usr/local/bin/retrosave-healthcheck
COPY --from=frontend-build /src/frontend/dist /app/frontend-dist
COPY contracts /app/contracts

ENV PORT=80
ENV AUTH_MODE=disabled
ENV SAVE_ROOT=/saves
ENV STATE_ROOT=/config
ENV FRONTEND_DIST_DIR=/app/frontend-dist
ENV CHEAT_PACK_ROOT=/app/contracts/cheats/packs

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD ["/usr/local/bin/retrosave-healthcheck"]

ENTRYPOINT ["/usr/local/bin/retrosave-api"]
