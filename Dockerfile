# syntax=docker/dockerfile:1

##
## Build
##
FROM --platform=$BUILDPLATFORM goreleaser/goreleaser:v1.22.1 AS build

ARG TARGETOS TARGETARCH
ARG BUILD_WITH_COVERAGE
ARG BUILD_SNAPSHOT=true

WORKDIR /app

COPY . .

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH goreleaser build --snapshot="${BUILD_SNAPSHOT}" --single-target -o extension
##
## Runtime
##
FROM alpine:3.18

LABEL "steadybit.com.discovery-disabled"="true"

ARG USERNAME=steadybit
ARG USER_UID=10000

RUN adduser -u $USER_UID -D $USERNAME

USER $USERNAME

WORKDIR /

COPY --from=build /app/extension /extension
COPY --from=build /app/licenses /licenses

EXPOSE 8085
EXPOSE 8086

ENTRYPOINT ["/extension"]
