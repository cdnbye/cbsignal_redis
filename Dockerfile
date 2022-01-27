FROM golang:1.16-alpine AS builder
WORKDIR /go/src/app
COPY . .
RUN go build -o cbsignal -v main.go

FROM ubuntu:18.04
WORKDIR /cbsignal_redis
COPY --from=builder /go/src/app/cbsignal .
RUN chmod +x cbsignal
CMD ["./cbsignal", "-c", "config/config.yaml"]