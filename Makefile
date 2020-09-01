generate:
	# Generate gogo, gRPC-Gateway, swagger, go-validators output.
	#
	# -I declares import folders, in order of importance
	# This is how proto resolves the protofile imports.
	# It will check for the protofile relative to each of these
	# folders and use the first one it finds.
	#
	# --gogo_out generates GoGo Protobuf output with gRPC plugin enabled.
	# --grpc-gateway_out generates gRPC-Gateway output.
	# --swagger_out generates an OpenAPI 2.0 specification for our gRPC-Gateway endpoints.
	# --govalidators_out generates Go validation files for our messages types, if specified.
	#
	# The lines starting with Mgoogle/... are proto import replacements,
	# which cause the generated file to import the specified packages
	# instead of the go_package's declared by the imported protof files.
	#
	# ./api is the output directory.
	protoc \
		-I api \
		-I vendor/github.com/grpc-ecosystem/grpc-gateway/ \
		-I vendor/github.com/gogo/googleapis/ \
		-I vendor/github.com/gogo/protobuf/ \
		-I vendor/github.com/coreos/etcd/raft/ \
		-I vendor/ \
		--gogo_out=plugins=grpc,paths=source_relative,\
Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/empty.proto=github.com/gogo/protobuf/types,\
Mgoogle/api/annotations.proto=github.com/gogo/googleapis/google/api,\
Mgoogle/protobuf/field_mask.proto=github.com/gogo/protobuf/types:\
./api/ \
		--grpc-gateway_out=allow_patch_feature=false,paths=source_relative,\
Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/empty.proto=github.com/gogo/protobuf/types,\
Mgoogle/api/annotations.proto=github.com/gogo/googleapis/google/api,\
Mgoogle/protobuf/field_mask.proto=github.com/gogo/protobuf/types:\
./api/ \
		--swagger_out=third_party/OpenAPI/ \
		--govalidators_out=gogoimport=true,paths=source_relative,\
Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/empty.proto=github.com/gogo/protobuf/types,\
Mgoogle/api/annotations.proto=github.com/gogo/googleapis/google/api,\
Mgoogle/protobuf/field_mask.proto=github.com/gogo/protobuf/types:\
./api/ \
		api/controller.proto

	# Workaround for https://github.com/grpc-ecosystem/grpc-gateway/issues/229.
	sed -i.bak "s/empty.Empty/types.Empty/g" api/controller.pb.gw.go && rm api/controller.pb.gw.go.bak

	# Generate static assets for OpenAPI UI
	statik -m -f -src third_party/OpenAPI/ -dest internal

install:
	go get \
		github.com/gogo/protobuf/protoc-gen-gogo \
		github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway \
		github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger \
		github.com/mwitkow/go-proto-validators/protoc-gen-govalidators \
		github.com/rakyll/statik
