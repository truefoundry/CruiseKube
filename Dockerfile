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
ARG VERSION=dev

ENV USAGETELEMETRY_PROVIDER_API_KEY=
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo \
	-ldflags "-X github.com/truefoundry/cruisekube/pkg/buildmetadata.Version=${VERSION} \
	-X github.com/truefoundry/cruisekube/pkg/buildmetadata.DefaultUsageTelemetryProviderAPIKey=${USAGETELEMETRY_PROVIDER_API_KEY}" \
	-o cruisekube ./cmd/cruisekube

# Runtime stage
FROM public.ecr.aws/docker/library/alpine:3.23
RUN apk --no-cache add ca-certificates tzdata sqlite
WORKDIR /app
COPY --from=builder /app/cruisekube .
COPY config.yaml /app/config.yaml
RUN mkdir -p /app/data
EXPOSE 8080
CMD ["./cruisekube"]
