package backup

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/fs"
	"slices"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/storage"
)

// uploadStream sends an archive in two chunks.
type uploadStream struct {
	grpc.ServerStream
	ctx  context.Context
	msgs []*noryxv1.UploadBackupRequest
	res  *noryxv1.UploadBackupResponse
}

func upload(ctx context.Context, header *noryxv1.UploadBackupHeader, archive []byte) *uploadStream {
	half := len(archive) / 2
	return &uploadStream{ctx: ctx, msgs: []*noryxv1.UploadBackupRequest{
		{Content: &noryxv1.UploadBackupRequest_Header{Header: header}},
		{Content: &noryxv1.UploadBackupRequest_Data{Data: archive[:half]}},
		{Content: &noryxv1.UploadBackupRequest_Data{Data: archive[half:]}},
	}}
}

func (s *uploadStream) Context() context.Context { return s.ctx }

func (s *uploadStream) Recv() (*noryxv1.UploadBackupRequest, error) {
	if len(s.msgs) == 0 {
		return nil, io.EOF
	}
	msg := s.msgs[0]
	s.msgs = s.msgs[1:]
	return msg, nil
}

func (s *uploadStream) SendAndClose(res *noryxv1.UploadBackupResponse) error {
	s.res = res
	return nil
}

type zipEntry struct {
	name, content string
	mode          fs.FileMode
}

func zipped(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		h.SetMode(e.mode | 0o644)
		w, err := zw.CreateHeader(h)
		check(t, err)
		_, err = w.Write([]byte(e.content))
		check(t, err)
	}
	check(t, zw.Close())
	return buf.Bytes()
}

// An uploaded backup is checked before it is kept, and restoring it brings its files but none
// of the secrets, forwarding settings or files of the agent of the server it came from.
func TestUploadBackup(t *testing.T) {
	path, _ := paperData(t)
	write(t, path+"/server.properties", "motd=hi\nrcon.password=ours\n")
	rt := &fakeRuntime{dir: path}
	s := NewService(rt, storage.New(t.TempDir()))
	ctx := t.Context()

	stream := upload(ctx, &noryxv1.UploadBackupHeader{ServerId: serverID, Label: "From the host"}, zipped(t,
		zipEntry{name: "world/level.dat", content: "theirs"}, zipEntry{name: "world/region/r.0.0.mca", content: "their chunks"},
		zipEntry{name: "server.properties", content: "motd=theirs\nrcon.password=theirs\nonline-mode=false\n"},
		zipEntry{name: ".rcon-cli.env", content: "password=theirs"}, zipEntry{name: "noryx-filesets.json", content: `{"marked":[]}`},
		zipEntry{name: "config/paper-global.yml", content: "proxies:\n  velocity:\n    enabled: true\n    secret: their-secret\n"},
		zipEntry{name: "plugins/Their.jar", content: "jar"},
	))
	check(t, s.UploadBackup(stream))
	b := stream.res.GetBackup()
	if !b.GetUntrusted() || b.GetLabel() != "From the host" || !slices.Equal(b.GetPaths(), []string{"config", "plugins", "server.properties", "world"}) {
		t.Fatalf("backup = %v", b)
	}

	// Archives with links, paths outside the data, odd names or too much data are refused.
	var tarGz bytes.Buffer
	gz := gzip.NewWriter(&tarGz)
	_, _ = gz.Write(make([]byte, 1024))
	check(t, gz.Close())
	for name, archive := range map[string][]byte{
		"zip slip":        zipped(t, zipEntry{name: "world/level.dat"}, zipEntry{name: "../../outside", content: "x"}),
		"link":            zipped(t, zipEntry{name: "world", content: "/etc", mode: fs.ModeSymlink}),
		"./ in names":     zipped(t, zipEntry{name: "./world/level.dat", content: "x"}),
		"only secrets":    zipped(t, zipEntry{name: ".rcon-cli.env", content: "password=x"}),
		"not a ZIP":       tarGz.Bytes(),
		"no archive":      []byte("hello"),
		"ratio of a bomb": zipped(t, zipEntry{name: "zeros", content: string(make([]byte, 65<<20))}),
	} {
		err := s.UploadBackup(upload(ctx, &noryxv1.UploadBackupHeader{ServerId: serverID}, archive))
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("upload of %s: %v", name, err)
		}
	}
	if list, _ := s.store.List(serverID); len(list) != 1 {
		t.Fatalf("backups = %v", list)
	}

	_, err := s.RestoreBackup(ctx, &noryxv1.RestoreBackupRequest{ServerId: serverID, BackupId: b.GetId(), Paths: []string{"world"}})
	check(t, err)
	if read(path, "world/region/r.0.0.mca") != "their chunks" || read(path, "server.properties") != "motd=hi\nrcon.password=ours\n" {
		t.Fatal("restoring the world restored more or less")
	}
	_, err = s.RestoreBackup(ctx, &noryxv1.RestoreBackupRequest{ServerId: serverID, BackupId: b.GetId()})
	check(t, err)
	for name, want := range map[string]string{
		"server.properties":       "motd=theirs\nrcon.password=ours\nonline-mode=true\n",
		".rcon-cli.env":           "secret",
		"noryx-filesets.json":     "{}",
		"plugins/Their.jar":       "jar",
		"plugins/LuckPerms.jar":   "",
		"config/paper-global.yml": "proxies:\n    velocity: {}\n",
		"bukkit.yml":              "a: 1\n", // not in the archive, so it stays
	} {
		if got := read(path, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
