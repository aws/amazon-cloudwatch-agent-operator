# Use the official Golang image as the base image
FROM golang:1.21-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the Go script into the container
COPY main.go .

# Install any required dependencies (use Go modules if needed)
ENV GOPROXY=direct
RUN apk add --no-cache git
RUN go mod init tls-endpoint-tester
RUN go mod tidy

# Build the Go binary
RUN go build -o main .


# Command to run the Go application
CMD ["./main"]
