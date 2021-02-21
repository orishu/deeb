module github.com/orishu/deeb

go 1.15

require (
	github.com/coreos/etcd v3.3.24+incompatible
	github.com/etcd-io/etcd v3.3.24+incompatible
	github.com/go-sql-driver/mysql v1.5.0
	github.com/gogo/gateway v1.1.0
	github.com/gogo/googleapis v1.4.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.4.3
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.1
	github.com/grpc-ecosystem/grpc-gateway v1.14.7
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/mattn/go-sqlite3 v1.14.2
	github.com/mwitkow/go-proto-validators v0.3.2
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1
	github.com/rakyll/statik v0.1.7
	github.com/stretchr/testify v1.6.1
	go.uber.org/fx v1.13.1
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	golang.org/x/tools v0.0.0-20200904185747-39188db58858 // indirect
	google.golang.org/grpc v1.31.1
	honnef.co/go/tools v0.0.1-2020.1.5 // indirect
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/cli-runtime v0.20.1
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920 // indirect
)

replace k8s.io/api => k8s.io/api v0.19.6

replace k8s.io/client-go => k8s.io/client-go v0.19.6
