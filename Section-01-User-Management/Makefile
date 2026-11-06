SERVICES := orch-user-access core-authentication core-user-manage
COMPOSE := docker compose --env-file .env -f deployment/compose.yaml
CLEAN_COMPOSE := docker compose --env-file /dev/null -f deployment/compose.yaml
HTTPYAC_VERSION ?= 6.16.7

.PHONY: build test unit-test vet check check-pkg check-contracts generate up down dev-env ensure-index integration mocks storage-integration clean pkg-test

build vet:
	@set -eu; for service in $(SERVICES); do $(MAKE) -C "$$service" $@; done

test:
	npx --yes httpyac@$(HTTPYAC_VERSION) send 'orch-user-access/tests/http/*.http' --all --output none --output-failed none --no-color

unit-test: check
	@set -eu; for service in $(SERVICES); do $(MAKE) -C "$$service" test; done

check: check-pkg check-contracts

check-pkg:
	@set -eu; temporary=$$(mktemp -d); trap 'rm -rf "$$temporary"' EXIT HUP INT TERM; \
	for service in $(SERVICES); do \
		find "$$service/pkg" -type f ! -name '*.pb.go' ! -name '*_test.go' | while IFS= read -r source; do \
			relative=$${source#$$service/pkg/}; destination="$$temporary/$$service/$$relative"; \
			mkdir -p "$$(dirname "$$destination")"; \
			sed "s|github.com/nutsp/7solutions-backend-challenge/$$service/|LOCAL_MODULE/|g" "$$source" > "$$destination"; \
		done; \
	done; \
	for service in core-authentication core-user-manage; do \
		diff -r "$$temporary/orch-user-access" "$$temporary/$$service"; \
	done; \
	echo 'PASS: reusable pkg code is synchronized across all three services'

check-contracts:
	@set -eu; temporary=$$(mktemp -d); trap 'rm -rf "$$temporary"' EXIT HUP INT TERM; \
	for contract in core-user-manage/user-service.proto core-authentication/authentication-service.proto; do \
		for service in $(SERVICES); do \
			sed 's/option go_package = .*;/option go_package = "LOCAL";/' "$$service/pkg/proto/$$contract" > "$$temporary/$$service"; \
		done; \
		for service in core-authentication core-user-manage; do diff -u "$$temporary/orch-user-access" "$$temporary/$$service"; done; \
	done; \
	echo 'PASS: domain protobuf contracts are synchronized'

generate: check-contracts
	@set -eu; for service in $(SERVICES); do $(MAKE) -C "$$service" generate; done

mocks:
	@set -eu; for service in $(SERVICES); do $(MAKE) -C "$$service" mocks; done

dev-env:
	@set -eu; umask 077; \
	if [ -e .env ] || [ -L .env ]; then echo '.env already exists; keeping existing secrets (skipped).'; exit 0; fi; \
	temporary=$$(mktemp .env.tmp.XXXXXX); trap 'rm -f "$$temporary"' EXIT HUP INT TERM; \
	{ \
		echo '# Local development only. Never commit this file.'; \
		for key in JWT_SECRET ORCH_SERVICE_TOKEN AUTH_SERVICE_TOKEN IDENTITY_MONGO_ROOT_PASSWORD IDENTITY_MONGO_PASSWORD AUTH_REDIS_PASSWORD; do \
			value=$$(openssl rand -hex 32); printf '%s=%s\n' "$$key" "$$value"; \
		done; \
		value=$$(openssl rand -base64 32); printf "USER_DATA_KEYS='{\"v1\":\"%s\"}'\n" "$$value"; \
		value=$$(openssl rand -base64 32); printf "USER_AUDIT_KEYS='{\"v1\":\"%s\"}'\n" "$$value"; \
		value=$$(openssl rand -base64 32); printf 'EMAIL_LOOKUP_KEY=%s\n' "$$value"; \
		echo 'GATEWAY_PORT=8080'; \
	} > "$$temporary"; \
	ln "$$temporary" .env; \
	echo 'Created .env with random secrets and mode 600.'

ensure-index:
	$(COMPOSE) run --rm --build ensure-index

up:
	$(COMPOSE) up -d --build --wait

down:
	$(COMPOSE) down

integration:
	$(MAKE) -C orch-user-access integration

storage-integration:
	$(MAKE) -C core-user-manage integration

# Cleanup only parses Compose; dummy values let it run even when .env is missing.
clean:
	@echo 'Removing project containers, images, volumes and local build artifacts (MongoDB/Redis data will be deleted).'
	@JWT_SECRET=cleanup IDENTITY_MONGO_ROOT_PASSWORD=cleanup IDENTITY_MONGO_PASSWORD=cleanup \
		AUTH_REDIS_PASSWORD=cleanup USER_DATA_KEYS=cleanup USER_AUDIT_KEYS=cleanup EMAIL_LOOKUP_KEY=cleanup \
		$(CLEAN_COMPOSE) --profile '*' down --volumes --remove-orphans --rmi all
	@set -eu; for service in $(SERVICES); do $(MAKE) -C "$$service" clean; done
	rm -rf ensure-index/bin graphify-out docs/code-graph