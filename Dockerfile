# syntax=docker/dockerfile:1

##
## Build
##
FROM goreleaser/goreleaser:v1.19.2 AS build

ARG BUILD_WITH_COVERAGE

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN goreleaser build --snapshot --single-target -o extension
##
## Runtime
##
FROM alpine:3.17

ARG USERNAME=steadybit
ARG USER_UID=10000

RUN adduser -u $USER_UID -D $USERNAME

USER $USERNAME

WORKDIR /

COPY --from=build /app/extension /extension

EXPOSE 8085
EXPOSE 8086

ENTRYPOINT ["/extension"]
