FROM golang:1.21 AS build-stage
# Note: this does not build the tracker JS files, which need to preexist in /static

ARG GIT_COMMIT
ARG GIT_BRANCH
ARG APP_VERSION
ARG TARGETPLATFORM # set by Docker Buildx

WORKDIR /app
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
RUN case "${TARGETPLATFORM}" in \
      "linux/amd64") \
        URL="https://github.com/sqlc-dev/sqlc/releases/download/v1.23.0/sqlc_1.23.0_linux_amd64.tar.gz" ;; \
      "linux/arm64") \
        URL="https://github.com/sqlc-dev/sqlc/releases/download/v1.23.0/sqlc_1.23.0_linux_arm64.tar.gz" ;; \
      *) echo "Unsupported platform: ${TARGETPLATFORM}"; exit 1 ;; \
    esac && \
    curl -L ${URL} | tar -xz -C /usr/local/bin sqlc

RUN [ -f geoip.mmdb ] || (curl -L https://download.db-ip.com/free/dbip-city-lite-2023-12.mmdb.gz | gunzip -c > geoip.mmdb)

COPY go.mod go.sum ./
RUN go mod download

COPY picolytics/ ./picolytics/
COPY cmd/ ./cmd/
RUN cd picolytics && sqlc generate

# Build the application
RUN cd cmd/picolytics && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.InjectedGitCommit=$GIT_COMMIT -X main.InjectedGitBranch=$GIT_BRANCH -X main.InjectedAppVersion=$APP_VERSION"

# Deploy the application binary into a lean image
FROM scratch
WORKDIR /
COPY --from=build-stage /app/cmd/picolytics/picolytics  /picolytics
COPY --from=build-stage /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-stage /app/geoip.mmdb geoip.mmdb
EXPOSE 8080
ENTRYPOINT ["/picolytics"]
