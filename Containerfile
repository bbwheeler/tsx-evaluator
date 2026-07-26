# Build stage
FROM golang:1.25-bookworm AS build
WORKDIR /src

# Install protoc from official release (Debian-packaged version is often outdated).
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl unzip && \
    curl -fsSL "https://github.com/protocolbuffers/protobuf/releases/download/v29.3/protoc-29.3-linux-x86_64.zip" -o /tmp/protoc.zip && \
    unzip -o /tmp/protoc.zip -d /usr/local && \
    rm /tmp/protoc.zip && \
    rm -rf /var/lib/apt/lists/*

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copy evaluator source and its tracker dependency (go.mod replace directive).
COPY tsx-evaluator /src/tsx-evaluator
COPY tsx-tracker /src/tsx-tracker
WORKDIR /src/tsx-evaluator

ENV PATH="/root/go/bin:${PATH}"
RUN protoc \
      --go_out=gen --go_opt=paths=source_relative \
      --go-grpc_out=gen --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false \
      -I proto proto/tsx/v1/evaluator.proto

RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /out/tsx-evaluator ./cmd/server

# Runtime stage
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/tsx-evaluator /tsx-evaluator
EXPOSE 50052
ENTRYPOINT ["/tsx-evaluator"]
