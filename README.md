# Mod.io Cache API for SkaterXL (Go & Dockerized)

A self-hosted Go API that fetches Skater XL mod and map data from Mod.io, caches it in memory, and serves it via a JSON API. Features scheduled cache refreshes. Designed for containerized deployment.

## Key Technologies

- Go (net/http, log/slog)
- Mod.io API
- Docker & Docker Compose
- Caddy (as reverse proxy for deployment)

## Deployed API (Target)

Once deployed, the API will be accessible at a URL like:
`https://api.skatebit.app`

## Quick Start / Local Development (Docker)

**Prerequisites:**

- Docker Desktop (or Docker Engine)
- Git
- A `.env` file created from `.env.example` with your `MODIO_API_KEY`.

**Steps:**

1.  Clone the repository:
    ```bash
    git clone [https://github.com/ShawnEdgell/modio-api-go.git](https://github.com/ShawnEdgell/modio-api-go.git) # Replace with your repo URL
    cd modio-api-go
    ```
2.  Create your `.env` file (copy from `.env.example`) and add your `MODIO_API_KEY` and set `PORT=8000`.
3.  Build the Docker image:
    ```bash
    docker build -t modio-api-go-local .
    ```
4.  Run the container (maps host port 8083 to container port 8000):
    ```bash
    docker run -d -p 8083:8000 --env-file .env --name test-modio-api modio-api-go-local
    ```
5.  Access locally at `http://localhost:8083`. For example: `http://localhost:8083/api/v1/skaterxl/maps`.

_(For native Go development: ensure Go is installed, set environment variables, then `go mod tidy && go run .`)_

## API Endpoints

- `GET /health`: Health check.
- `GET /api/v1/skaterxl/maps`: Returns cached Skater XL maps.
- `GET /api/v1/skaterxl/scripts`: Returns cached Skater XL script mods.
- `GET /`: Redirects to `https://www.skatebit.app` (or an API status message).

_(Responses include `itemType`, `lastUpdated`, `count`, and `items` array.)_

## Environment Variables

Configure via a `.env` file (see `.env.example`):

- `MODIO_API_KEY` (Required): Your Mod.io API key.
- `PORT`: Internal port the Go application listens on (default: `8000`).
- `CACHE_REFRESH_INTERVAL_HOURS`: Defaults to `6`.
- `MODIO_GAME_ID`: Defaults to `629`.
- `MODIO_API_DOMAIN`: Defaults to `g-9677.modapi.io`.

## Deployment

Deployed on a VPS using Docker. The application is containerized with the `Dockerfile` in this project and managed by a central `docker-compose.yml` on the server. Caddy serves as the reverse proxy, providing HTTPS.

## Key Project Files

- `main.go`: Application entry point.
- `internal/`: Contains core application packages (config, modio client, cache, scheduler, HTTP server).
- `go.mod`, `go.sum`: Go module files.
- `Dockerfile`: For building the application's Docker image.
- `.dockerignore`: Specifies files to ignore during Docker build.
- `.env.example`: Template for required environment variables.
- `README.md`: This file.
