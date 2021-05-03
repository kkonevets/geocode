.DEFAULT_GOAL := all
.PHONY: proto all

ifndef $(GOPATH)
    GOPATH=$(shell go env GOPATH)
    export GOPATH
endif

PROTOC_GEN_GO := $(GOPATH)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(GOPATH)/bin/protoc-gen-go-grpc
PG := ./proto/geocoding
PX := ./proto/xxdb
BUILD_DIR := ./build

# If $GOPATH/bin/protoc-gen-go does not exist, we'll run this command to install it.
$(PROTOC_GEN_GO):
	go get -u google.golang.org/protobuf/cmd/protoc-gen-go

$(PROTOC_GEN_GO_GRPC):
	go get -u google.golang.org/grpc/cmd/protoc-gen-go-grpc

$(PG)/geocoding.pb.go: $(PG)/geocoding.proto | $(PROTOC_GEN_GO)
	protoc --proto_path=$(PG) --go_out=$(PG) \
	--go_opt=paths=source_relative $<

$(PG)/geocoding_grpc.pb.go: $(PG)/geocoding.proto | $(PROTOC_GEN_GO_GRPC)
	protoc --proto_path=$(PG) --go-grpc_out=$(PG) \
	--go-grpc_opt=paths=source_relative $<

$(PX)/xxdb.pb.go: $(PX)/xxdb.proto | $(PROTOC_GEN_GO)
	protoc --proto_path=$(PX) --go_out=$(PX) \
	--go_opt=paths=source_relative $<

proto: $(PG)/geocoding.pb.go $(PG)/geocoding_grpc.pb.go $(PX)/xxdb.pb.go

$(BUILD_DIR):
	mkdir -p $@

all: proto $(BUILD_DIR)
	go build -o ./build ./...

clean:
	rm -f $(PG)/*.pb.go $(PX)/*.pb.go build/*
