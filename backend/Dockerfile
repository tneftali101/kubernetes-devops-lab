# Stage 1: Build the Go application
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Download Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Compile our Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o creator-studio-api .

# Stage 2: Create a smaller runtime image
FROM alpine:3.22

WORKDIR /app

# Install certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Copy only the compiled application
COPY --from=builder /app/creator-studio-api .

# Run without root privileges
USER 10001:10001

# Document the API port
EXPOSE 8080

# Start the application
CMD ["./creator-studio-api"]
