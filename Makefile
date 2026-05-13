.PHONY: build build-server build-client test test-coverage lint docker-build docker-push run clean release gen-cert

APP_NAME_SERVER = gos3
APP_NAME_CLIENT = gos3c
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -w -s -X main.version=$(VERSION)

all: build

build: build-server build-client

build-server:
	@echo "Building server..."
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME_SERVER) ./cmd/server

build-client:
	@echo "Building client..."
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME_CLIENT) ./cmd/client

test:
	go test -v -race ./...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	golangci-lint run

docker-build:
	docker build -t $(APP_NAME_SERVER):latest -f deploy/Dockerfile .

docker-push:
	# Add docker push logic here

run: build-server
	./$(APP_NAME_SERVER) --config deploy/config.example.yaml

clean:
	rm -f $(APP_NAME_SERVER) $(APP_NAME_CLIENT) coverage.out

release:
	goreleaser release --snapshot --clean

gen-cert:
	@echo "Generating self-signed certificate for development..."
	mkdir -p certs
	go run $(shell go env GOROOT)/src/crypto/tls/generate_cert.go --rsa-bits 2048 --host localhost,127.0.0.1,::1
	mv cert.pem certs/server.crt
	mv key.pem certs/server.key
	@echo "Done. Certificates saved in certs/ directory."
