# syntax=docker/dockerfile:1

##
## Build
##
FROM golang:1.20-alpine AS build

ARG NAME
ARG VERSION
ARG REVISION

WORKDIR /app

RUN apk add build-base
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build \
    -ldflags="\
    -X 'github.com/steadybit/extension-kit/extbuild.ExtensionName=${NAME}' \
    -X 'github.com/steadybit/extension-kit/extbuild.Version=${VERSION}' \
    -X 'github.com/steadybit/extension-kit/extbuild.Revision=${REVISION}'" \
    -o ./extension

##
## Runtime
##
FROM alpine:3.16

ARG USERNAME=steadybit
ARG USER_UID=10000

RUN adduser -u $USER_UID -D $USERNAME

USER $USERNAME

WORKDIR /

COPY --from=build /app/extension /extension

EXPOSE 8085
EXPOSE 8086

ENTRYPOINT ["/extension"]
