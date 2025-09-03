generate:
	# Generate modern protobuf, gRPC, gRPC-Gateway, and OpenAPI output
	# Include paths for googleapis and grpc-gateway proto files
	protoc \
		-I api \
		-I $$(go env GOMODCACHE)/github.com/grpc-ecosystem/grpc-gateway/v2@v2.18.0 \
		-I $$(go env GOMODCACHE)/github.com/grpc-ecosystem/grpc-gateway@v1.16.0/third_party/googleapis \
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
