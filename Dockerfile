# Start from a lightweight Go image
FROM golang:1.23.2-alpine

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum first to enable dependency caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go application
RUN go build -o manga-scraper-api main.go

# Expose the application port
EXPOSE 8000

# Run the Go application
CMD ["./manga-scraper-api"]
