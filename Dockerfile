# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY . .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build .

FROM alpine:latest

RUN apk update \
  && apk add --no-cache ca-certificates apprise

WORKDIR /overpush
COPY --from=builder /app/overpush .

VOLUME ["/etc/overpush.toml"]

EXPOSE 8080
ENTRYPOINT ["./overpush"]

