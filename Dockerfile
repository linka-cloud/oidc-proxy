FROM golang:alpine as builder

RUN apk add git ca-certificates

WORKDIR /oidc-proxy

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY cmd cmd
COPY pkg pkg

RUN go build -o oidc-proxy ./cmd/oidc-proxy

FROM alpine

COPY --from=builder /etc/ca-certificates /etc/ca-certificates
COPY --from=builder /oidc-proxy/oidc-proxy /usr/bin/oidc-proxy

EXPOSE 8888

ENTRYPOINT ["/usr/bin/oidc-proxy"]
