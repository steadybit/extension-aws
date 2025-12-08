# syntax=docker/dockerfile:1

##
## Build
##
FROM --platform=$BUILDPLATFORM goreleaser/goreleaser:v2.13.0 AS build

ARG TARGETOS
ARG TARGETARCH
ARG BUILD_WITH_COVERAGE
ARG BUILD_SNAPSHOT=true
ARG SKIP_LICENSES_REPORT=false
ARG VERSION=unknown
ARG REVISION=unknown

WORKDIR /app

COPY . .
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH goreleaser build --snapshot="${BUILD_SNAPSHOT}" --single-target -o extension

##
## Runtime
##
FROM alpine:3.23

ARG VERSION=unknown
ARG REVISION=unknown

LABEL "steadybit.com.discovery-disabled"="true"
LABEL "version"="${VERSION}"
LABEL "revision"="${REVISION}"
RUN echo "$VERSION" > /version.txt && echo "$REVISION" > /revision.txt

ARG USERNAME=steadybit
ARG USER_UID=10000

RUN adduser -u $USER_UID -D $USERNAME

USER $USER_UID

WORKDIR /

COPY --from=build /app/extension /extension
COPY --from=build /app/licenses /licenses

EXPOSE 8085
EXPOSE 8086

ENTRYPOINT ["/extension"]
