FROM golang:1.16-alpine AS builder
WORKDIR /go/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o cbsignal -v main.go

FROM ubuntu:18.04
WORKDIR /cbsignal_redis
COPY --from=builder /go/src/app/cbsignal .
COPY config .
RUN chmod +x cbsignal
CMD ["./cbsignal", "-c", "config/config.yaml"]