FROM golang:1.22-alpine as buildbase

RUN apk add git build-base

WORKDIR /go/src/github.com/debabky/relayer
COPY vendor .
COPY . .

RUN GOOS=linux go build  -o /usr/local/bin/proof-verification-relayer /go/src/github.com/debabky/relayer


FROM alpine:3.9

COPY --from=buildbase /usr/local/bin/proof-verification-relayer /usr/local/bin/proof-verification-relayer
RUN apk add --no-cache ca-certificates

ENTRYPOINT ["proof-verification-relayer"]
