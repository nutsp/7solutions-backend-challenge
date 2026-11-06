MODULE := $(shell go list -m)
PROTOC ?= protoc
PROTOC_GEN_GO_VERSION ?= v1.36.12
PROTOC_GEN_GO_GRPC_VERSION ?= v1.6.1
PROTO_FILES := pkg/proto/core-user-manage/user-service.proto pkg/proto/core-authentication/authentication-service.proto

.PHONY: test generate gen-proto check-contracts tools

test:
	go test -race -count=1 ./...

check-contracts:
	$(MAKE) -C ../.. check-contracts

tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

generate gen-proto: check-contracts tools
	cd .. && PATH="$$(go env GOPATH)/bin:$$PATH" $(PROTOC) --go_out=. --go_opt=module=$(MODULE) --go-grpc_out=. --go-grpc_opt=module=$(MODULE) $(PROTO_FILES)
