FROM golang:1.22-alpine

WORKDIR /app

# Set Go environment variables
ENV GOPROXY=direct
ENV GO111MODULE=on

# Install necessary packages
RUN apk add --no-cache git build-base

# Copy the go mod and sum files first to cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application from the correct directory
RUN go build  -o=/bin/main .

# Copy configuration file
COPY ./config.yaml /config.yaml

CMD ["/bin/main"]
