FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download
RUN go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -v -o /modio-api-app .

FROM alpine:latest

WORKDIR /app/

RUN apk --no-cache add ca-certificates curl tzdata
ENV TZ=Etc/UTC

COPY --from=builder /modio-api-app .

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD curl -f http://localhost:8000/health || exit 1

CMD ["./modio-api-app"]
