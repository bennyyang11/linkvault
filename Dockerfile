FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o linkvault .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && \
    wget -q https://github.com/replicatedhq/troubleshoot/releases/latest/download/support-bundle_linux_amd64.tar.gz -O /tmp/sb.tar.gz && \
    tar xzf /tmp/sb.tar.gz -C /usr/local/bin support-bundle && \
    rm /tmp/sb.tar.gz
COPY --from=builder /app/linkvault /usr/local/bin/linkvault
EXPOSE 8080
CMD ["linkvault"]
