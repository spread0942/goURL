FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gourl ./cmd/gourl

FROM alpine:3.23
RUN apk add --no-cache ca-certificates \
    && addgroup -g 10001 gourl \
    && adduser -D -u 10001 -G gourl gourl \
    && mkdir /data && chown gourl:gourl /data && chmod 700 /data
COPY --from=build /out/gourl /usr/local/bin/gourl
USER gourl
ENV TERM=xterm-256color
VOLUME ["/data"]
ENTRYPOINT ["gourl", "--data-dir", "/data"]