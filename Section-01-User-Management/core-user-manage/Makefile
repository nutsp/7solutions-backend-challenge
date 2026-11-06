SERVICE := core-user-manage
COMPOSE_PROJECT_NAME ?= seven-solutions
PROTOC ?= protoc
MOCKERY_VERSION ?= v3.9.0
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
	@set -eu; root=$$(cd .. && pwd); \
	architecture=$$(docker image inspect mongo:8.0 --format '{{.Architecture}}'); \
	case "$$architecture" in arm64|amd64) ;; *) echo 'Unsupported MongoDB container architecture' >&2; exit 1 ;; esac; \
	temporary=$$(mktemp -d); trap 'rm -rf "$$temporary"' EXIT HUP INT TERM; \
	. "$$root/.env"; \
	export CGO_ENABLED=0 GOOS=linux GOARCH="$$architecture"; \
	export MONGO_TEST_PASSWORD="$$IDENTITY_MONGO_ROOT_PASSWORD"; \
	export MONGO_TEST_URI='mongodb://user-mongo:27017/?replicaSet=rs0'; \
	export ENSURE_INDEX_BIN=/ensure-index ENSURE_INDEX_CONFIG=/ensure-index-config.yaml; \
	(cd "$$root/ensure-index" && go build -o "$$temporary/ensure-index" ./cmd); \
	for module in ensure-index core-user-manage; do \
		case "$$module" in ensure-index) package=./internal/indexes ;; *) package=./internal/repository/mongodb ;; esac; \
		(cd "$$root/$$module" && go test -c -tags=integration -o "$$temporary/$$module.test" "$$package"); \
		docker run --rm --network '$(COMPOSE_PROJECT_NAME)_user-db' \
			--env MONGO_TEST_URI --env MONGO_TEST_PASSWORD --env ENSURE_INDEX_BIN --env ENSURE_INDEX_CONFIG \
			--mount "type=bind,src=$$temporary/$$module.test,dst=/test,readonly" \
			--mount "type=bind,src=$$temporary/ensure-index,dst=/ensure-index,readonly" \
			--mount "type=bind,src=$$root/ensure-index/config/config.yaml,dst=/ensure-index-config.yaml,readonly" \
			mongo:8.0 /test -test.v; \
	done
