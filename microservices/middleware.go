// Middleware for grpc
package micromiddleware

import (
	config "chat_room_go/utils/conf"
	"chat_room_go/utils/logs"
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/tap"
)

var logger *zap.SugaredLogger

func init() {
	logger = logs.InitDirLogger(config.Config.MicroserviceMiddleware.PathToLogs)
}

var TokenAuth string

// Logs any incoming/outcoming request
func LogInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.Errorf("Retrieving metadata is failed")
	}

	reply, err := handler(ctx, req)
	if err != nil {
		logger.Errorf("Error, during handling request. %s", err)
	}

	logger.Infow("Recieved request",
		"method", info.FullMethod,
		"request", fmt.Sprintf("%#v", req),
		"reply", fmt.Sprintf("%#v", reply),
		"time", time.Since(start),
		"md", md,
		"error", err,
	)

	return reply, err
}

// Checks authorization header and token
func AuthInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {

	if err := authorize(ctx); err != nil {
		return nil, err
	}

	// Calls the handler
	h, err := handler(ctx, req)

	return h, err
}

// Authorizes the token received from Metadata
func authorize(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "Retrieving metadata is failed")
	}

	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) != 1 {
		return status.Errorf(codes.Unauthenticated, "Authorization token is not supplied")
	}

	token := authHeader[0]
	err := validateToken(token)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, err.Error())
	}
	return nil
}

// Validates the token
func validateToken(token string) error {
	if token != TokenAuth {
		return status.Errorf(codes.Unauthenticated, "Wrong token")
	}
	return nil
}

// NiceMD is a convenience wrapper definiting extra functions on the metadata.
type NiceMD metadata.MD

// ExtractIncoming extracts an inbound metadata from the server-side context.
//
// This function always returns a NiceMD wrapper of the metadata.MD, in case the context doesn't have metadata it returns
// a new empty NiceMD.
func ExtractIncoming(ctx context.Context) NiceMD {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return NiceMD(metadata.Pairs())
	}
	return NiceMD(md)
}

// The passed in `Context` will contain the gRPC metadata MD object
type AuthFunc func(ctx context.Context) (context.Context, error)

// TODO: check avaibility, starts before anything: grpc.InTapHandle(rateLimiter)
func RateLimiter(ctx context.Context, info *tap.Info) (context.Context, error) {
	fmt.Printf("--\nCheck ratelim for %s\n", info.FullMethodName)
	return ctx, nil
}
