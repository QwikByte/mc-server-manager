package progress

import (
	"context"
	"io"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// A watcher that starts before the named call gets its progress and ends with it.
func TestWatchProgress(t *testing.T) {
	r := NewRegistry()
	stream := &fakeStream{ctx: t.Context(), header: make(chan struct{}), sent: make(chan *noryxv1.WatchProgressResponse, 100)}
	watched := make(chan error)
	go func() { watched <- r.WatchProgress(&noryxv1.WatchProgressRequest{OperationId: "op-1"}, stream) }()
	<-stream.header // like the master, which starts the call once the watcher follows it

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(noryxv1.OperationKey, "op-1"))
	_, err := r.intercept(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		Step(ctx, "image", 100)
		Set(ctx, 50, 100)
		Step(ctx, "copy", 3<<20)
		_, err := io.Copy(io.Discard, Reader(ctx, strings.NewReader(strings.Repeat("x", 3<<20))))
		return nil, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-watched; err != nil {
		t.Fatal(err)
	}
	close(stream.sent)
	var last *noryxv1.WatchProgressResponse
	for msg := range stream.sent {
		last = msg
	}
	if last.GetStep() != "copy" || last.GetDone() != 3<<20 || last.GetTotal() != 3<<20 {
		t.Fatalf("last progress = %v", last)
	}
	if len(r.trackers) != 0 {
		t.Fatal("trackers are left")
	}
}

// Calls that aren't named report nothing.
func TestUnnamed(t *testing.T) {
	r := NewRegistry()
	_, _ = r.intercept(t.Context(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		if Active(ctx) {
			t.Error("an unnamed call is watched")
		}
		Step(ctx, "image", 1) // does nothing
		return nil, nil
	})
}

type fakeStream struct {
	grpc.ServerStreamingServer[noryxv1.WatchProgressResponse]
	ctx    context.Context
	header chan struct{}
	sent   chan *noryxv1.WatchProgressResponse
}

func (f *fakeStream) Context() context.Context { return f.ctx }

func (f *fakeStream) SendHeader(metadata.MD) error {
	close(f.header)
	return nil
}

func (f *fakeStream) Send(m *noryxv1.WatchProgressResponse) error {
	f.sent <- m
	return nil
}
