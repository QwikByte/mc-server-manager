// Package files is the file manager of a server: the master streams files between the
// browser and the agent of the server's node, which confines all paths.
package files

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"time"

	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const (
	chunkSize      = 256 << 10
	maxUploadBytes = 16 << 30 // the agent enforces the same limit
	opTimeout      = 30 * time.Second
)

// Nodes provides connections to node agents.
type Nodes interface {
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

type Handler struct{ nodes Nodes }

func NewHandler(nodes Nodes) *Handler { return &Handler{nodes: nodes} }

func (h *Handler) Register(mux access.Mux) {
	const base = "/api/nodes/{node}/servers/{id}/files"
	read, write := access.OnServer(access.FilesRead), access.OnServer(access.FilesWrite)
	mux.Handle("GET "+base, read, h.list)
	mux.Handle("DELETE "+base, write, h.delete)
	mux.Handle("GET "+base+"/content", read, h.download)
	mux.Handle("PUT "+base+"/content", write, h.upload)
	mux.Handle("GET "+base+"/archive", read, h.archive)
	mux.Handle("POST "+base+"/directories", write, h.createDirectory)
	mux.Handle("POST "+base+"/move", write, h.move)
}

type fileView struct {
	Name      string    `json:"name"`
	Directory bool      `json:"directory"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
}

func toView(f *noryxv1.FileInfo) fileView {
	return fileView{f.GetName(), f.GetDirectory(), f.GetSize(), time.Unix(f.GetModifiedUnix(), 0)}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), opTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	res, err := c.ListFiles(ctx, &noryxv1.ListFilesRequest{ServerId: r.PathValue("id"), Path: r.URL.Query().Get("path")})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	files := make([]fileView, 0, len(res.GetFiles()))
	for _, f := range res.GetFiles() {
		files = append(files, toView(f))
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"files": files, "truncated": res.GetTruncated()})
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	c, err := h.client(r.Context(), r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	p := r.URL.Query().Get("path")
	stream, err := c.ReadFile(r.Context(), &noryxv1.ReadFileRequest{ServerId: r.PathValue("id"), Path: p})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	first, err := stream.Recv() // errors such as a missing file arrive here
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.Attachment(w, path.Base(p))
	w.Header().Set("Content-Length", strconv.FormatInt(first.GetSize(), 10))
	httpapi.Relay(w, first, stream)
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	c, err := h.client(r.Context(), r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	p := r.URL.Query().Get("path")
	stream, err := c.ArchiveDirectory(r.Context(), &noryxv1.ArchiveDirectoryRequest{ServerId: r.PathValue("id"), Path: p, HideSecrets: true})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	first, err := stream.Recv()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	name := path.Base("/" + p)
	if name == "/" {
		name = "server"
	}
	httpapi.Attachment(w, name+".zip")
	httpapi.Relay(w, first, stream)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context()) // cancelling discards the partial file
	defer cancel()
	c, err := h.client(ctx, r)
	var file *noryxv1.FileInfo
	if err == nil {
		file, err = Write(ctx, c, &noryxv1.WriteFileHeader{
			ServerId: r.PathValue("id"), Path: r.URL.Query().Get("path"), Overwrite: r.URL.Query().Get("overwrite") == "true",
			Size: max(r.ContentLength, 0),
		}, http.MaxBytesReader(w, r.Body, maxUploadBytes))
	}
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusRequestEntityTooLarge, "Files can have up to %d GB.", maxUploadBytes>>30))
	case err != nil:
		httpapi.WriteError(w, r, err)
	default:
		httpapi.WriteJSON(w, http.StatusCreated, toView(file))
	}
}

// Write creates or replaces a file of a server, as header describes it, with content.
func Write(ctx context.Context, c noryxv1.FileServiceClient, header *noryxv1.WriteFileHeader, content io.Reader) (*noryxv1.FileInfo, error) {
	stream, err := c.WriteFile(ctx)
	if err != nil {
		return nil, err
	}
	err = stream.Send(&noryxv1.WriteFileRequest{Content: &noryxv1.WriteFileRequest_Header{Header: header}})
	buf := make([]byte, chunkSize)
	for err == nil {
		var n int
		n, err = io.ReadFull(content, buf)
		if n > 0 {
			if sendErr := stream.Send(&noryxv1.WriteFileRequest{Content: &noryxv1.WriteFileRequest_Data{Data: buf[:n]}}); sendErr != nil {
				err = sendErr
			}
		}
	}
	// The content ended, or the agent ended the stream (io.EOF), whose reply tells why.
	if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	res, err := stream.CloseAndRecv()
	return res.GetFile(), err
}

func (h *Handler) createDirectory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	h.unary(w, r, &req, func(ctx context.Context, c noryxv1.FileServiceClient, id string) error {
		_, err := c.CreateDirectory(ctx, &noryxv1.CreateDirectoryRequest{ServerId: id, Path: req.Path})
		return err
	})
}

func (h *Handler) move(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	h.unary(w, r, &req, func(ctx context.Context, c noryxv1.FileServiceClient, id string) error {
		_, err := c.MoveFile(ctx, &noryxv1.MoveFileRequest{ServerId: id, From: req.From, To: req.To})
		return err
	})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	h.unary(w, r, nil, func(ctx context.Context, c noryxv1.FileServiceClient, id string) error {
		_, err := c.DeleteFile(ctx, &noryxv1.DeleteFileRequest{ServerId: id, Path: r.URL.Query().Get("path")})
		return err
	})
}

// unary reads an optional JSON body into req and runs an operation without result.
func (h *Handler) unary(w http.ResponseWriter, r *http.Request, req any, op func(context.Context, noryxv1.FileServiceClient, string) error) {
	ctx, cancel := context.WithTimeout(r.Context(), opTimeout)
	defer cancel()
	var err error
	if req != nil {
		err = httpapi.ReadJSON(w, r, req)
	}
	var c noryxv1.FileServiceClient
	if err == nil {
		c, err = h.client(ctx, r)
	}
	if err == nil {
		err = op(ctx, c, r.PathValue("id"))
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) client(ctx context.Context, r *http.Request) (noryxv1.FileServiceClient, error) {
	conn, err := h.nodes.Conn(ctx, r.PathValue("node"))
	if err != nil {
		return nil, err
	}
	return noryxv1.NewFileServiceClient(conn), nil
}
