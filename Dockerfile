FROM docker.io/library/golang:1.18-alpine as builder

MAINTAINER Jack Murdock <jack_murdock@comcast.com>

WORKDIR /src

ARG VERSION
ARG GITCOMMIT
ARG BUILDTIME


RUN apk add --no-cache --no-progress \
    ca-certificates \
    make \
    git \
    openssh \
    gcc \
    libc-dev \
    upx

RUN go install github.com/geofffranks/spruce/cmd/spruce@v1.29.0 && chmod +x /go/bin/spruce
COPY . .
#RUN make test release
RUN make release

FROM alpine:3.12.1

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /src/themis /src/themis.yaml /src/deploy/packaging/entrypoint.sh /go/bin/spruce /src/Dockerfile /src/NOTICE /src/LICENSE /src/CHANGELOG.md /
COPY --from=builder /src/deploy/packaging/themis_spruce.yaml /tmp/themis_spruce.yaml

RUN mkdir /etc/themis/ && touch /etc/themis/themis.yaml && chmod 666 /etc/themis/themis.yaml

USER nobody

ENTRYPOINT ["/entrypoint.sh"]

EXPOSE 6500
EXPOSE 6501
EXPOSE 6502
EXPOSE 6503

CMD ["/themis"]
