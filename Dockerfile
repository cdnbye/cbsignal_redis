FROM golang:1.16-alpine AS builder
WORKDIR /go/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o cbsignal -v main.go

FROM ubuntu:18.04
WORKDIR /cbsignal_redis
COPY --from=builder /go/src/app/cbsignal /go/bin/cbsignal
COPY config .
ENV PATH="/go/bin:${PATH}"
CMD ["cbsignal", "-c", "config/config.yaml"]