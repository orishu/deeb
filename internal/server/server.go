package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/http"

	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/insecure"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/transport"
	"github.com/pkg/errors"
	"github.com/rakyll/statik/fs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

// ServerParams is the group of parameters for the gRPC/HTTP server.
type ServerParams struct {
	Addr        string
	Port        string
	GatewayPort string
	PrivateKey  []byte
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
	/*
		TODO: don't use insecure

		certificates := make([]tls.Certificate, 0, 1)
		certPool := x509.NewCertPool()
		if params.PrivateKey != nil {
			cert, err := insecure.CreateSelfSignedCertificate(params.PrivateKey, params.Addr)
			if err != nil {
				panic(err)
			}
			certificates = append(certificates, *cert)

			parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				panic(err)
			}
			certPool.AddCert(parsedCert)
		}

		creds := credentials.NewTLS(&tls.Config{
			ServerName:         params.Addr,
			Certificates:       certificates,
			RootCAs:            certPool,
			InsecureSkipVerify: true,
		})
	*/
	grpcServer := grpc.NewServer(
		grpc.Creds(grpcinsecure.NewCredentials()),
		grpc.UnaryInterceptor(grpc_validator.UnaryServerInterceptor()),
		grpc.StreamInterceptor(grpc_validator.StreamServerInterceptor()),
	)

	cservice := controlService{node: node}
	pb.RegisterControlServiceServer(grpcServer, &cservice)
	rservice := raftService{node: node, transportMgr: transportMgr, logger: logger}
	pb.RegisterRaftServer(grpcServer, &rservice)

	mux := http.NewServeMux()

	return &Server{
		node:       node,
		addr:       params.Addr,
		port:       params.Port,
		gwPort:     params.GatewayPort,
		grpcServer: grpcServer,
		httpServer: &http.Server{
			Addr: fmt.Sprintf(":%s", params.GatewayPort),
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
	// gRPC Gateway v2 uses protojson by default with proper settings

	listenAddr := fmt.Sprintf(":%s", s.port)
	addrport := fmt.Sprintf("%s:%s", s.addr, s.port)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to listen on %s", listenAddr)
	}

	// Serve gRPC Server
	s.logger.Infof("Serving gRPC on https://%s", addrport)
	go func() {
		err := s.grpcServer.Serve(lis)
		if err != nil {
			s.logger.Errorf("error serving gRPC: %+v", err)
		}
	}()

	// See https://github.com/grpc/grpc/blob/master/doc/naming.md
	// for gRPC naming standard information.
	dialAddr := fmt.Sprintf("passthrough://localhost/%s", addrport)
	conn, err := grpc.DialContext(
		ctx,
		dialAddr,
		//grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(insecure.CertPool, "")),
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		return errors.Wrapf(err, "failed to dial server %s", dialAddr)
	}

	gwmux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				EmitUnpopulated: true,
				Indent:          "  ",
				UseProtoNames:   true,
			},
		}),
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
		err := s.node.Start(ctx)
		if err != nil {
			s.logger.Errorf("failed to start node", err)
		}
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
