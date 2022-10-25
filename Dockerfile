# syntax=docker/dockerfile:1
FROM golang:bullseye AS builder
WORKDIR /opt/build
COPY golang/ /opt/build/
WORKDIR /opt/build
RUN CGO_ENABLED=0 go build -a -o jtl-stats-processor

FROM ubuntu:20.04
WORKDIR /opt
COPY --from=builder /opt/build/jtl-stats-processor ./
COPY entrypoint.sh /
ENTRYPOINT ["/opt/jtl-stats-processor"]
