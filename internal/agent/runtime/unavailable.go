package runtime

import (
	"context"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
)

const probeTimeout = 3 * time.Second

// Interceptors tell callers when a call failed because the runtime is down, instead of passing
// on the error of its client, e.g. "failed to connect to the docker API at unix://…".
func Interceptors(rt Runtime) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			res, err := handler(ctx, req)
			return res, explain(ctx, rt, err)
		}),
		grpc.ChainStreamInterceptor(func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return explain(ss.Context(), rt, handler(srv, ss))
		}),
	}
}

// explain replaces an unexpected error if the runtime can't be reached.
func explain(ctx context.Context, rt Runtime, err error) error {
	if code := status.Code(err); code != codes.Internal && code != codes.Unknown {
		return err
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), probeTimeout)
	defer cancel()
	if _, down := rt.Info(ctx); down == nil {
		return err
	}
	st, _ := status.New(codes.Unavailable, "Docker isn't running on the node, or the agent can't connect to it.").
		WithDetails(&errdetails.ErrorInfo{Reason: noryxv1.ReasonRuntimeUnavailable, Domain: "noryx.v1"})
	return st.Err()
}
