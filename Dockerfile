# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install ca-certificates for HTTPS and tzdata for timezone support; explicitly
# upgrade the CA bundle so the eMudhra/emSign distrust (ca-certificates
# 20261020) is picked up even on a stale base image
RUN apk add --no-cache --upgrade ca-certificates ca-certificates-bundle tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o birthday-bot ./cmd/bot

# Runtime stage
FROM scratch

# Copy CA certificates and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/birthday-bot /birthday-bot

ENTRYPOINT ["/birthday-bot"]
