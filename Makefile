generate:
	# Generate modern protobuf, gRPC, gRPC-Gateway, and OpenAPI output
	# Include paths are determined dynamically from go modules
	$(eval GRPC_GATEWAY_V2_PATH := $(shell go list -mod=mod -m -f '{{.Dir}}' github.com/grpc-ecosystem/grpc-gateway/v2))
	$(eval GOOGLEAPIS_PATH := $(shell go list -mod=mod -m -f '{{.Dir}}' github.com/googleapis/googleapis))
	protoc \
		-I api \
		-I $(GRPC_GATEWAY_V2_PATH) \
		-I $(GOOGLEAPIS_PATH) \
		--go_out=./api/ \
		--go_opt=paths=source_relative \
		--go-grpc_out=./api/ \
		--go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=./api/ \
		--grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=third_party/OpenAPI/ \
		api/controller.proto

	# Generate static assets for OpenAPI UI
	statik -m -f -src third_party/OpenAPI/ -dest internal

install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	go install github.com/rakyll/statik@latest
