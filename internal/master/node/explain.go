package node

import (
	"context"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
)

// explained makes calls to the agent of a node fail with messages that name the node if it
// can't be used: its agent can't be reached, or Docker isn't running on it. The panel shows them
// where the node may not be obvious, e.g. on a network.
func (s *Service) explained(id string) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoke grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return s.explain(id, invoke(ctx, method, req, reply, cc, opts...))
		}),
		grpc.WithChainStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			stream, err := streamer(ctx, desc, cc, method, opts...)
			if err != nil {
				return nil, s.explain(id, err)
			}
			return explainedStream{stream, s, id}, nil
		}),
	}
}

type explainedStream struct {
	grpc.ClientStream
	svc  *Service
	node string
}

func (e explainedStream) RecvMsg(m any) error {
	return e.svc.explain(e.node, e.ClientStream.RecvMsg(m))
}

func (s *Service) explain(id string, err error) error {
	st, _ := status.FromError(err)
	if st.Code() != codes.Unavailable {
		return err
	}
	name := "The node"
	if n, getErr := s.Get(context.Background(), id); getErr == nil {
		name = n.Name
	}
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok && info.GetReason() == noryxv1.ReasonRuntimeUnavailable {
			return status.Errorf(codes.Unavailable, "Docker isn't running on %s, or its agent can't connect to it.", name)
		}
	}
	return status.Errorf(codes.Unavailable, "%s can't be reached. Check that its agent is running and that the master can connect to it.", name)
}
