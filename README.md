# Mod.io API Cache for SkaterXL (Go, Redis & Dockerized)

A self-hosted Go API that fetches Skater XL mod/map data from Mod.io, caches it in Redis, and serves it via JSON. Uses an event-driven mechanism for updates and periodic full syncs.

## Core Technologies

- Go (net/http, slog, go-chi/chi)
- Redis
- Mod.io API (Events API)
- Docker

## Quick Start (Local Docker)

**Prerequisites:** Docker, Git, `.env` file (from `.env.example`) with `MODIO_API_KEY`.

1.  **Clone:** `git clone <your_repo_url> && cd modio-api-go`
2.  **Setup `.env`:** Copy `.env.example` to `.env`, add your `MODIO_API_KEY`.
3.  **Run Redis (if not already running):**
    ```bash
    docker run -d --name local-redis -p 6379:6379 redis:alpine
    ```
4.  **Build API Image:**
    ```bash
    docker build -t modio-api-go-local .
    ```
5.  **Run API Container:**
    ```bash
    docker run -d -p 8080:8000 \
      --name test-modio-api \
      -e PORT="8000" \
      -e MODIO_API_KEY="your_modio_api_key" \
      -e REDIS_ADDR="host.docker.internal:6379" \
      modio-api-go-local
    ```
    (Adjust `8080` if that host port is taken. Use `host.docker.internal` for `REDIS_ADDR` on Docker Desktop; for Linux, use a shared Docker network and the Redis container name, e.g., `redis:6379`).
6.  **Access:** `http://localhost:8080/api/v1/skaterxl/maps`

## Key API Endpoints

- `GET /health`: Health check (includes Redis).
- `GET /api/v1/skaterxl/maps`: Get Skater XL maps.
- `GET /api/v1/skaterxl/scripts`: Get Skater XL script mods.
- `GET /api/v1/skaterxl/maps/autocomplete?prefix={p}`: Autocomplete map titles.
- `GET /api/v1/skaterxl/scripts/autocomplete?prefix={p}`: Autocomplete script titles.

## Essential Environment Variables

(See `.env.example` for all variables and defaults)

- `MODIO_API_KEY`: **Required**.
- `PORT`: Internal port for the Go app (default: `8000`).
- `REDIS_ADDR`: Redis server address (default: `localhost:6379`).
- `LIGHTWEIGHT_CHECK_INTERVAL_MINUTES`: Event polling interval (default: `15`).
- `CACHE_REFRESH_INTERVAL_HOURS`: Full sync interval (default: `6`).

## Deployment

Containerize using the provided `Dockerfile`. Deploy on a VPS using Docker, ideally with Docker Compose to manage the API and Redis services. Use a reverse proxy (e.g., Caddy, Nginx) for HTTPS.

## Project Structure Highlights

- `main.go`: Entry point.
- `internal/`:
  - `config/`: Environment configuration.
  - `modio/`: Mod.io API client & types.
  - `repository/`: Redis data operations.
  - `scheduler/`: Data sync logic.
  - `server/`: HTTP server, routing, handlers.
- `Dockerfile`: Builds the production image.
