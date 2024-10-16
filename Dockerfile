FROM golang:1.23-alpine as builder
ARG version

WORKDIR /go/src/app
ADD . /go/src/app

RUN  CGO_ENABLED=0 \
       go build -ldflags "-s -w" -o app

FROM gcr.io/distroless/base-debian10

COPY --from=builder /go/src/app/app /
ENTRYPOINT ["/app"]
