package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/http"

	"github.com/gogo/gateway"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/insecure"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/transport"
	"github.com/pkg/errors"
	"github.com/rakyll/statik/fs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ServerParams is the group of parameters for the gRPC/HTTP server.
type ServerParams struct {
	Addr        string
	Port        string
	GatewayPort string
}

// Server is the gRPC and HTTP server
type Server struct {
	node       *nd.Node
	addr       string
	port       string
	gwPort     string
	grpcServer *grpc.Server
	httpServer *http.Server
	httpMux    *http.ServeMux
	logger     *zap.SugaredLogger
}

// New creates a new combined gRPC and HTTP server
func New(
	params ServerParams,
	node *nd.Node,
	transportMgr *transport.TransportManager,
	logger *zap.SugaredLogger,
) *Server {
	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&insecure.Cert)),
		grpc.UnaryInterceptor(grpc_validator.UnaryServerInterceptor()),
		grpc.StreamInterceptor(grpc_validator.StreamServerInterceptor()),
	)

	cservice := controlService{node: node}
	pb.RegisterControlServiceServer(grpcServer, &cservice)
	rservice := raftService{nodeID: node.GetID(), transportMgr: transportMgr}
	pb.RegisterRaftServer(grpcServer, &rservice)

	mux := http.NewServeMux()

	return &Server{
		node:       node,
		addr:       params.Addr,
		port:       params.Port,
		gwPort:     params.GatewayPort,
		grpcServer: grpcServer,
		httpServer: &http.Server{
			Addr: fmt.Sprintf("%s:%s", params.Addr, params.GatewayPort),
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{insecure.Cert},
			},
			Handler: mux,
		},
		httpMux: mux,
		logger:  logger,
	}
}

// Start starts the gRPC server
func (s *Server) Start(ctx context.Context) error {
	jsonpb := &gateway.JSONPb{
		EmitDefaults: true,
		Indent:       "  ",
		OrigName:     true,
	}

	addr := fmt.Sprintf("%s:%s", s.addr, s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrapf(err, "failed to listen on %s", addr)
	}

	// Serve gRPC Server
	s.logger.Infof("Serving gRPC on https://%s", addr)
	go func() {
		err := s.grpcServer.Serve(lis)
		if err != nil {
			s.logger.Errorf("error serving gRPC: %+v", err)
		}
	}()

	// See https://github.com/grpc/grpc/blob/master/doc/naming.md
	// for gRPC naming standard information.
	dialAddr := fmt.Sprintf("passthrough://localhost/%s", addr)
	conn, err := grpc.DialContext(
		ctx,
		dialAddr,
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(insecure.CertPool, "")),
		grpc.WithBlock(),
	)
	if err != nil {
		return errors.Wrapf(err, "failed to dial server %s", dialAddr)
	}

	gwmux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, jsonpb),
		// This is necessary to get error details properly
		// marshalled in unary requests.
		runtime.WithProtoErrorHandler(runtime.DefaultHTTPProtoErrorHandler),
	)
	err = pb.RegisterControlServiceHandler(ctx, gwmux, conn)
	if err != nil {
		return errors.Wrap(err, "failed to register control gateway")
	}
	err = pb.RegisterRaftHandler(ctx, gwmux, conn)
	if err != nil {
		return errors.Wrap(err, "failed to register raft gateway")
	}

	s.httpMux.Handle("/", gwmux)
	err = serveOpenAPI(s.httpMux)
	if err != nil {
		return errors.Wrap(err, "failed to serve OpenAPI UI")
	}

	s.logger.Infof("Serving gRPC-Gateway on https://%s", s.httpServer.Addr)
	s.logger.Infof("Serving OpenAPI Documentation on https://%s/openapi-ui/", s.httpServer.Addr)

	go func() {
		s.node.Start(ctx)
	}()

	go func() {
		s.httpServer.ListenAndServeTLS("", "")
	}()

	return nil
}

// Stop stops the combined gRPC HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.node.Stop()
	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		return err
	}
	s.grpcServer.GracefulStop()
	return nil
}

func serveOpenAPI(mux *http.ServeMux) error {
	mime.AddExtensionType(".svg", "image/svg+xml")

	statikFS, err := fs.New()
	if err != nil {
		return err
	}

	// Expose files in static on <host>/openapi-ui
	fileServer := http.FileServer(statikFS)
	prefix := "/openapi-ui/"
	mux.Handle(prefix, http.StripPrefix(prefix, fileServer))
	return nil
}
