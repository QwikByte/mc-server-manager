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

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
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

func (h *Handler) Register(mux *http.ServeMux) {
	const base = "/api/nodes/{node}/servers/{id}/files"
	mux.HandleFunc("GET "+base, h.list)
	mux.HandleFunc("DELETE "+base, h.delete)
	mux.HandleFunc("GET "+base+"/content", h.download)
	mux.HandleFunc("PUT "+base+"/content", h.upload)
	mux.HandleFunc("GET "+base+"/archive", h.archive)
	mux.HandleFunc("POST "+base+"/directories", h.createDirectory)
	mux.HandleFunc("POST "+base+"/move", h.move)
}

type fileView struct {
	Name      string    `json:"name"`
	Directory bool      `json:"directory"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
}

func toView(f *mcsmv1.FileInfo) fileView {
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
	res, err := c.ListFiles(ctx, &mcsmv1.ListFilesRequest{ServerId: r.PathValue("id"), Path: r.URL.Query().Get("path")})
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
	stream, err := c.ReadFile(r.Context(), &mcsmv1.ReadFileRequest{ServerId: r.PathValue("id"), Path: p})
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
	stream, err := c.ArchiveDirectory(r.Context(), &mcsmv1.ArchiveDirectoryRequest{ServerId: r.PathValue("id"), Path: p})
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
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	stream, err := c.WriteFile(ctx)
	if err == nil {
		err = stream.Send(&mcsmv1.WriteFileRequest{Content: &mcsmv1.WriteFileRequest_Header{Header: &mcsmv1.WriteFileHeader{
			ServerId: r.PathValue("id"), Path: r.URL.Query().Get("path"), Overwrite: r.URL.Query().Get("overwrite") == "true",
		}}})
	}
	if err == nil {
		err = sendBody(http.MaxBytesReader(w, r.Body, maxUploadBytes), stream)
	}
	var res *mcsmv1.WriteFileResponse
	if err == nil || errors.Is(err, io.EOF) { // io.EOF: the agent ended the stream, its reply tells why
		res, err = stream.CloseAndRecv()
	}
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusRequestEntityTooLarge, "Files can have up to %d GB.", maxUploadBytes>>30))
	case err != nil:
		httpapi.WriteError(w, r, err)
	default:
		httpapi.WriteJSON(w, http.StatusCreated, toView(res.GetFile()))
	}
}

func sendBody(body io.Reader, stream mcsmv1.FileService_WriteFileClient) error {
	buf := make([]byte, chunkSize)
	for {
		n, err := io.ReadFull(body, buf)
		if n > 0 {
			if err := stream.Send(&mcsmv1.WriteFileRequest{Content: &mcsmv1.WriteFileRequest_Data{Data: buf[:n]}}); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (h *Handler) createDirectory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	h.unary(w, r, &req, func(ctx context.Context, c mcsmv1.FileServiceClient, id string) error {
		_, err := c.CreateDirectory(ctx, &mcsmv1.CreateDirectoryRequest{ServerId: id, Path: req.Path})
		return err
	})
}

func (h *Handler) move(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	h.unary(w, r, &req, func(ctx context.Context, c mcsmv1.FileServiceClient, id string) error {
		_, err := c.MoveFile(ctx, &mcsmv1.MoveFileRequest{ServerId: id, From: req.From, To: req.To})
		return err
	})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	h.unary(w, r, nil, func(ctx context.Context, c mcsmv1.FileServiceClient, id string) error {
		_, err := c.DeleteFile(ctx, &mcsmv1.DeleteFileRequest{ServerId: id, Path: r.URL.Query().Get("path")})
		return err
	})
}

// unary reads an optional JSON body into req and runs an operation without result.
func (h *Handler) unary(w http.ResponseWriter, r *http.Request, req any, op func(context.Context, mcsmv1.FileServiceClient, string) error) {
	ctx, cancel := context.WithTimeout(r.Context(), opTimeout)
	defer cancel()
	var err error
	if req != nil {
		err = httpapi.ReadJSON(w, r, req)
	}
	var c mcsmv1.FileServiceClient
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

func (h *Handler) client(ctx context.Context, r *http.Request) (mcsmv1.FileServiceClient, error) {
	conn, err := h.nodes.Conn(ctx, r.PathValue("node"))
	if err != nil {
		return nil, err
	}
	return mcsmv1.NewFileServiceClient(conn), nil
}
