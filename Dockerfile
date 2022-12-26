# syntax=docker/dockerfile:1

ARG ALPINE_VERSION=3.17
ARG GOLANG_VERSION=1.18

FROM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION} as build
ADD . /app
WORKDIR /app
RUN go mod download
RUN go build -o /spotify-status .

FROM alpine:${ALPINE_VERSION}
USER 1000
COPY --link --from=build /spotify-status /app/spotify-status
EXPOSE 3000
ENTRYPOINT [ "/app/spotify-status" ]
