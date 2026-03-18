# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /src

# Cache dependency downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a fully static binary.
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/mock-ai ./cmd/mock-ai

# Runtime stage — scratch for smallest possible image.
FROM scratch

# Import CA certs and timezone data from builder.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary and default fixtures.
COPY --from=builder /bin/mock-ai /bin/mock-ai
COPY fixtures/ /fixtures/

EXPOSE 8080

ENTRYPOINT ["/bin/mock-ai"]
