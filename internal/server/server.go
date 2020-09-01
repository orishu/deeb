package server

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"os"

	"github.com/gogo/gateway"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/insecure"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/rakyll/statik/fs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

var (
	gRPCPort    = flag.Int("grpc-port", 10000, "The gRPC server port")
	gatewayPort = flag.Int("gateway-port", 11000, "The gRPC-Gateway server port")
)

var log grpclog.LoggerV2

func init() {
	log = grpclog.NewLoggerV2(os.Stdout, ioutil.Discard, ioutil.Discard)
	grpclog.SetLoggerV2(log)
}

// Server is the gRPC and HTTP server
type Server struct {
	node       *nd.Node
	grpcServer *grpc.Server
	httpServer *http.Server
	httpMux    *http.ServeMux
}

// New creates a new combined gRPC and HTTP server
func New(node *nd.Node) *Server {
	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&insecure.Cert)),
		grpc.UnaryInterceptor(grpc_validator.UnaryServerInterceptor()),
		grpc.StreamInterceptor(grpc_validator.StreamServerInterceptor()),
	)

	cservice := controlService{}
	pb.RegisterControlServiceServer(grpcServer, &cservice)
	rservice := raftService{node: node}
	pb.RegisterRaftServer(grpcServer, &rservice)

	mux := http.NewServeMux()

	return &Server{
		node:       node,
		grpcServer: grpcServer,
		httpServer: &http.Server{
			Addr: fmt.Sprintf("localhost:%d", *gatewayPort),
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{insecure.Cert},
			},
			Handler: mux,
		},
		httpMux: mux,
	}
}

// Start starts the gRPC server
func (s *Server) Start(ctx context.Context) {
	jsonpb := &gateway.JSONPb{
		EmitDefaults: true,
		Indent:       "  ",
		OrigName:     true,
	}

	addr := fmt.Sprintf("localhost:%d", *gRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	// Serve gRPC Server
	log.Info("Serving gRPC on https://", addr)
	go func() {
		err := s.grpcServer.Serve(lis)
		if err != nil {
			log.Errorf("error serving gRPC: %+v", err)
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
		log.Fatalln("Failed to dial server:", err)
	}

	gwmux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, jsonpb),
		// This is necessary to get error details properly
		// marshalled in unary requests.
		runtime.WithProtoErrorHandler(runtime.DefaultHTTPProtoErrorHandler),
	)
	err = pb.RegisterControlServiceHandler(ctx, gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register gateway:", err)
	}

	s.httpMux.Handle("/", gwmux)
	err = serveOpenAPI(s.httpMux)
	if err != nil {
		log.Fatalln("Failed to serve OpenAPI UI")
	}

	log.Info("Serving gRPC-Gateway on https://", s.httpServer.Addr)
	log.Info("Serving OpenAPI Documentation on https://", s.httpServer.Addr, "/openapi-ui/")
	err = s.httpServer.ListenAndServeTLS("", "")
	if err != http.ErrServerClosed {
		log.Errorf("http listening error: %+v")
	}
}

// Stop stops the combined gRPC HTTP server
func (s *Server) Stop(ctx context.Context) error {
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
