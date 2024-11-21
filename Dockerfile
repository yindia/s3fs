# syntax=docker/dockerfile:1.2
FROM cgr.dev/chainguard/go as build

WORKDIR /work

# Use build args for cache keys
ARG CACHEBUST=1

# Copy only go.mod and go.sum for dependency caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy the rest of the application code
COPY . ./

# Build CLI
FROM build as cli
RUN go build -o /usr/local/bin/s3fs ./main.go


# Final image for CLI
FROM cgr.dev/chainguard/go as cli-final
COPY --from=cli /usr/local/bin/s3fs /usr/local/bin/s3fs
ENTRYPOINT ["s3fs"]

