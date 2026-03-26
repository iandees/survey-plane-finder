FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o survey-plane-finder .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/survey-plane-finder /usr/local/bin/survey-plane-finder
ENTRYPOINT ["survey-plane-finder"]
