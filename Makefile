.PHONY: proto proto-protoc tidy run build docker-up docker-down podman-up podman-down podman-build quadlet-install quadlet-build quadlet-enable

proto:
	buf generate

proto-protoc:
	protoc \
		--go_out=gen --go_opt=paths=source_relative \
		--go-grpc_out=gen --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false \
		-I proto proto/tsx/v1/evaluator.proto

tidy:
	go mod tidy

build: proto
	go build -o bin/tsx-evaluator ./cmd/server

run: proto
	go run ./cmd/server

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v

podman-build:
	podman-compose -f podman-compose.yml build

podman-up:
	podman-compose -f podman-compose.yml up -d

podman-down:
	podman-compose -f podman-compose.yml down

quadlet-install:
	./deploy/install.sh

quadlet-build:
	podman build --no-cache -f Containerfile -t localhost/tsx-evaluator:latest ..

quadlet-enable: quadlet-build
	systemctl --user daemon-reload
	systemctl --user start tsx-evaluator-build.service
	systemctl --user enable --now tsx-evaluator.service
