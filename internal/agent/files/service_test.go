package files

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// An editor's save doesn't replace a file that changed since it was opened, unless asked to.
func TestWriteExpectedVersion(t *testing.T) {
	rt := dataRuntime{dir: t.TempDir()}
	svc, id := NewService(rt), runtime.NewID()
	file := filepath.Join(rt.dir, id, "server.properties")
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("motd=A\nrcon.password=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	opened := read(t, svc, id, "server.properties")
	if opened.GetSize() == 0 || opened.GetModifiedUnixNano() == 0 {
		t.Fatalf("version = %v", opened)
	}
	saved, err := write(svc, id, "server.properties", opened, "motd=B\nrcon.password=<hidden>\n")
	if err != nil {
		t.Fatal(err)
	}
	if saved.GetVersion().GetSize() != int64(len("motd=B\nrcon.password=secret\n")) {
		t.Fatalf("version after saving = %v", saved.GetVersion())
	}

	// Another save based on the first version fails, also once the file is gone.
	if _, err := write(svc, id, "server.properties", opened, "motd=C\n"); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("save of an old version: %v", err)
	}
	modified := time.Unix(0, saved.GetVersion().GetModifiedUnixNano()).Add(time.Second)
	if err := os.Chtimes(file, modified, modified); err != nil {
		t.Fatal(err)
	}
	if _, err := write(svc, id, "server.properties", saved.GetVersion(), "motd=C\n"); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("save after the file was touched: %v", err)
	}
	if _, err := write(svc, id, "gone.yml", opened, "a: b\n"); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("save of a file that is gone: %v", err)
	}
	if got, _ := os.ReadFile(file); string(got) != "motd=B\nrcon.password=secret\n" {
		t.Fatalf("file = %q", got)
	}

	// Without an expected version, as from older masters, it is replaced.
	if _, err := write(svc, id, "server.properties", nil, "motd=D\nrcon.password=<hidden>\n"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); string(got) != "motd=D\nrcon.password=secret\n" {
		t.Fatalf("file = %q", got)
	}
}

func read(t *testing.T, svc *Service, id, name string) *noryxv1.FileVersion {
	t.Helper()
	stream := &readStream{ctx: t.Context()}
	if err := svc.ReadFile(&noryxv1.ReadFileRequest{ServerId: id, Path: name}, stream); err != nil {
		t.Fatal(err)
	}
	return stream.first.GetVersion()
}

func write(svc *Service, id, name string, expected *noryxv1.FileVersion, content string) (*noryxv1.WriteFileResponse, error) {
	stream := &writeStream{ctx: context.Background(), msgs: []*noryxv1.WriteFileRequest{
		{Content: &noryxv1.WriteFileRequest_Header{Header: &noryxv1.WriteFileHeader{ServerId: id, Path: name, Overwrite: true, Expected: expected}}},
		{Content: &noryxv1.WriteFileRequest_Data{Data: []byte(content)}},
	}}
	err := svc.WriteFile(stream)
	return stream.res, err
}

// dataRuntime only has the data of servers, each in a folder named after it.
type dataRuntime struct {
	runtime.Runtime
	dir string
}

func (r dataRuntime) Data(_ context.Context, id string) (*datadir.Dir, error) {
	return datadir.Open(filepath.Join(r.dir, id))
}

type readStream struct {
	grpc.ServerStream
	ctx   context.Context
	first *noryxv1.ReadFileResponse
}

func (s *readStream) Context() context.Context { return s.ctx }

func (s *readStream) Send(res *noryxv1.ReadFileResponse) error {
	if s.first == nil {
		s.first = res
	}
	return nil
}

type writeStream struct {
	grpc.ServerStream
	ctx  context.Context
	msgs []*noryxv1.WriteFileRequest
	res  *noryxv1.WriteFileResponse
}

func (s *writeStream) Context() context.Context { return s.ctx }

func (s *writeStream) Recv() (*noryxv1.WriteFileRequest, error) {
	if len(s.msgs) == 0 {
		return nil, io.EOF
	}
	msg := s.msgs[0]
	s.msgs = s.msgs[1:]
	return msg, nil
}

func (s *writeStream) SendAndClose(res *noryxv1.WriteFileResponse) error {
	s.res = res
	return nil
}
