package node

import (
	"context"
	"errors"
	"io"

	"google.golang.org/grpc"
)

// Relay downloads a file from one agent and uploads it to another, after a header, and
// returns the answer of the upload. Agents never connect to each other. If the download
// fails, the upload is cancelled, so that the agent discards what it received.
func Relay[D any, PD interface {
	*D
	GetData() []byte
}, Req, Res any](
	ctx context.Context,
	download func(context.Context) (grpc.ServerStreamingClient[D], error),
	upload func(context.Context, ...grpc.CallOption) (grpc.ClientStreamingClient[Req, Res], error),
	header *Req,
	data func([]byte) *Req,
	progress func(int),
) (*Res, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	down, err := download(ctx)
	if err != nil {
		return nil, err
	}
	up, err := upload(ctx)
	if err == nil {
		err = up.Send(header)
	}
	for err == nil {
		var chunk *D
		if chunk, err = down.Recv(); errors.Is(err, io.EOF) {
			return up.CloseAndRecv()
		}
		if err != nil {
			return nil, err // cancels the upload
		}
		if err = up.Send(data(PD(chunk).GetData())); err == nil {
			progress(len(PD(chunk).GetData()))
		}
	}
	if errors.Is(err, io.EOF) { // the agent ended the upload; its answer tells why
		return up.CloseAndRecv()
	}
	return nil, err
}

// chunkSize is the size of the chunks Upload sends.
const chunkSize = 256 << 10

// Upload sends content to an agent in chunks, after a header, and returns the agent's answer,
// e.g. an archive that a browser uploads. If reading content fails, the caller cancels ctx,
// so that the agent discards what it received.
func Upload[Req, Res any](
	ctx context.Context,
	upload func(context.Context, ...grpc.CallOption) (grpc.ClientStreamingClient[Req, Res], error),
	header *Req,
	data func([]byte) *Req,
	content io.Reader,
) (*Res, error) {
	up, err := upload(ctx)
	if err == nil {
		err = up.Send(header)
	}
	buf := make([]byte, chunkSize)
	for err == nil {
		var n int
		n, err = io.ReadFull(content, buf)
		if n > 0 {
			if sendErr := up.Send(data(buf[:n])); sendErr != nil {
				err = sendErr
			}
		}
	}
	// The content ended, or the agent ended the upload (io.EOF), whose answer tells why.
	if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return up.CloseAndRecv()
}
