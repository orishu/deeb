generate:
	# Generate modern protobuf, gRPC, gRPC-Gateway, and OpenAPI output.
	#
	# -I declares import folders for proto imports.
	# --go_out generates standard Go protobuf output.
	# --go-grpc_out generates Go gRPC service code.
	# --grpc-gateway_out generates gRPC-Gateway reverse proxy.
	# --openapiv2_out generates OpenAPI 2.0 specification.
	#
	# ./api is the output directory.
	protoc \
		-I api \
		-I vendor/github.com/grpc-ecosystem/grpc-gateway/v2 \
		-I vendor/github.com/gogo/googleapis \
		-I vendor/github.com/gogo/protobuf \
		-I vendor/github.com/coreos/etcd/raft \
		-I vendor \
		--go_out=./api/ \
		--go_opt=paths=source_relative \
		--go_opt=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations \
		--go_opt=Mgoogle/api/http.proto=google.golang.org/genproto/googleapis/api/annotations \
		--go_opt=Mgithub.com/coreos/etcd/raft/raftpb/raft.proto=go.etcd.io/etcd/raft/v3/raftpb \
		--go-grpc_out=./api/ \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_opt=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations \
		--go-grpc_opt=Mgoogle/api/http.proto=google.golang.org/genproto/googleapis/api/annotations \
		--go-grpc_opt=Mgithub.com/coreos/etcd/raft/raftpb/raft.proto=go.etcd.io/etcd/raft/v3/raftpb \
		--grpc-gateway_out=./api/ \
		--grpc-gateway_opt=paths=source_relative \
		--grpc-gateway_opt=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations \
		--grpc-gateway_opt=Mgoogle/api/http.proto=google.golang.org/genproto/googleapis/api/annotations \
		--grpc-gateway_opt=Mgithub.com/coreos/etcd/raft/raftpb/raft.proto=go.etcd.io/etcd/raft/v3/raftpb \
		--openapiv2_out=third_party/OpenAPI/ \
		--openapiv2_opt=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations \
		--openapiv2_opt=Mgoogle/api/http.proto=google.golang.org/genproto/googleapis/api/annotations \
		--openapiv2_opt=Mgithub.com/coreos/etcd/raft/raftpb/raft.proto=go.etcd.io/etcd/raft/v3/raftpb \
		api/controller.proto

	# Generate static assets for OpenAPI UI
	statik -m -f -src third_party/OpenAPI/ -dest internal

install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	go install github.com/rakyll/statik@latest
