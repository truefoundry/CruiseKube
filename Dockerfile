# Build stage
FROM public.ecr.aws/docker/library/golang:1.25.9-alpine3.23 AS builder
RUN apk add --no-cache git gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY ./cmd ./cmd
COPY ./pkg ./pkg

# Baked into the API (e.g. /config version). CI passes VERSION to match the image tag:
# release: charts/cruisekube/values.yaml cruisekubeController.image.tag; main: github.sha.
ENV VERSION=dev
ARG USAGETELEMETRY_PROVIDER_API_KEY
RUN if [ -n "${VERSION}" ]; then echo "build: version (VERSION build-arg): present"; else echo "build: version (VERSION build-arg): empty"; fi && \
	if [ -n "${USAGETELEMETRY_PROVIDER_API_KEY}" ]; then echo "build: usage telemetry provider api key: present"; else echo "build: usage telemetry provider api key: empty"; fi

RUN CGO_ENABLED=1 GOOS=linux go build -a \
    -ldflags "\
        -s -w \
        -X github.com/truefoundry/cruisekube/pkg/buildmetadata.Version=${VERSION} \
        -X github.com/truefoundry/cruisekube/pkg/buildmetadata.DefaultUsageTelemetryProviderAPIKey=${USAGETELEMETRY_PROVIDER_API_KEY}" \
    -o cruisekube \
    ./cmd/cruisekube

# Runtime stage
FROM public.ecr.aws/docker/library/alpine:3.23
RUN apk --no-cache add ca-certificates tzdata sqlite
WORKDIR /app
COPY --from=builder /app/cruisekube .
COPY config.yaml /app/config.yaml
RUN mkdir -p /app/data
EXPOSE 8080
CMD ["./cruisekube"]
