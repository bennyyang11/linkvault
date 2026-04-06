FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o linkvault .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/linkvault /usr/local/bin/linkvault
EXPOSE 8080
CMD ["linkvault"]
