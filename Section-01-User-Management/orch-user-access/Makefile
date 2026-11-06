SERVICE := orch-user-access
PROTOC ?= protoc
MOCKERY_VERSION ?= v3.9.0
HTTPYAC_VERSION ?= 6.16.7
COMPOSE := docker compose --env-file ../.env -f ../deployment/compose.yaml

.PHONY: build run test vet check mocks generate pkg-test clean rebuild integration

build:
	go build -trimpath -o bin/server ./cmd

run:
	go run ./cmd

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

check:
	$(MAKE) -C .. check

mocks:
	go run github.com/vektra/mockery/v3@$(MOCKERY_VERSION) --config .mockery.yaml

generate:
	$(MAKE) -C pkg gen-proto PROTOC="$(PROTOC)"

pkg-test:
	$(MAKE) -C pkg test

clean:
	rm -rf bin coverage.out coverage.html

rebuild:
	$(COMPOSE) up -d --build --no-deps $(SERVICE)

integration:
	npx --yes httpyac@$(HTTPYAC_VERSION) send 'tests/http/*.http' --all --output none --output-failed none --no-color
	go test -tags=integration -count=1 -v ./tests/integration
