package runtime

import (
	"context"
	"errors"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

const probeTimeout = 3 * time.Second

// ErrWrongRuntime is returned while the socket of the runtime answers as another runtime than
// the agent is set to use, e.g. Podman behind Docker's socket, or as a version too old.
var ErrWrongRuntime = errors.New("the agent can't use this runtime")

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

// explain replaces an unexpected error if the runtime can't be reached, or is another one.
func explain(ctx context.Context, rt Runtime, err error) error {
	if code := status.Code(err); code != codes.Internal && code != codes.Unknown {
		return err
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), probeTimeout)
	defer cancel()
	_, down := rt.Info(ctx)
	switch {
	case down == nil:
		return err
	case errors.Is(down, ErrWrongRuntime):
		return status.Error(codes.FailedPrecondition, down.Error())
	}
	st, _ := status.New(codes.Unavailable, noryxv1.RuntimeTitle(rt.Name())+" isn't running on the node, or the agent can't connect to it.").
		WithDetails(&errdetails.ErrorInfo{
			Reason: noryxv1.ReasonRuntimeUnavailable, Domain: "noryx.v1", Metadata: map[string]string{noryxv1.MetadataRuntime: rt.Name()},
		})
	return st.Err()
}
