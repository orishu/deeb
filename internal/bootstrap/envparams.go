package bootstrap

import (
	"os"

	"github.com/orishu/deeb/internal/transport"
	"go.uber.org/zap"
)

type GRPCPortType string

// NewEnvParams creates the bootstrap params from the environment
func NewEnvParams(grpcPort GRPCPortType, clientFactory transport.ClientFactory, logger *zap.SugaredLogger) Params {
	return Params{
		ClusterName:            os.Getenv("CLUSTER_NAME"),
		NodeName:               os.Getenv("NODE_NAME"),
		DirWithNodeIDFile:      "/var/lib/mysql",
		ClusterSize:            3,
		GRPCPort:               string(grpcPort),
		TransportClientFactory: clientFactory,
		Logger:                 logger,
	}
}
