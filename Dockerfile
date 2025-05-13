# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy go.mod and go.sum to leverage Docker layer caching.
# Run 'go mod tidy' locally before building to ensure these are up-to-date.
COPY go.mod ./
COPY go.sum ./ 
RUN go mod download

# Copy the rest of your application source code
COPY . .

# Build the Go application, naming the binary 'modio-api-app'
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /modio-api-app . 

# Stage 2: Create a minimal final image from alpine
FROM alpine:latest
WORKDIR /app/

# Add ca-certificates for making HTTPS calls from within the app if necessary
# AND add curl for the healthcheck command
RUN apk --no-cache add ca-certificates curl

# Copy only the compiled application binary from the builder stage
COPY --from=builder /modio-api-app .

# Expose the port your Go API listens on (e.g., 8000, set by PORT env var)
EXPOSE 8000

# Command to run the application when the container starts
# The application will listen on the port specified by the PORT environment variable.
CMD ["./modio-api-app"]
