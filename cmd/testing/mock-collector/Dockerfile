# Use the official Golang image as the base image
FROM golang:1.21-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the Go script into the container
COPY main.go .

# Install any required dependencies (use Go modules if needed)
RUN go mod init tls-endpoint-tester
RUN go mod tidy
RUN apk add --no-cache git
RUN #export GOPROXY=direct && go install github.com/go-delve/delve/cmd/dlv@latest

# Build the Go binary
RUN go build -o main .


# Command to run the Go application
CMD ["./main"]
