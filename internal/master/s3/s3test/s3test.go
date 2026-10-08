// Package s3test serves a bucket of S3-compatible storage in memory over HTTPS, for tests. It
// knows the requests of package s3, addressed in the path, and checks the hashes they sign.
package s3test

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Server is a bucket, at https://<Endpoint>/<Bucket>/.
type Server struct {
	*httptest.Server
	Bucket   string
	Endpoint string

	mu      sync.Mutex
	objects map[string]Object
	uploads map[string]*upload // the multipart uploads in progress
	// fail, if set, makes the requests fail that it returns a status for.
	fail func(r *http.Request) int
}

// Object is a stored object and the headers of the request that created it.
type Object struct {
	Data   []byte
	Header http.Header
}

type upload struct {
	parts  map[int][]byte
	header http.Header
}

// New serves an empty bucket until the test ends.
func New(t *testing.T) *Server {
	s := &Server{Bucket: "backups", objects: map[string]Object{}, uploads: map[string]*upload{}}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(s.serve))
	s.Endpoint = strings.TrimPrefix(s.URL, "https://")
	t.Cleanup(s.Close)
	return s
}

// Fail makes the requests fail that f returns a status other than 0 for.
func (s *Server) Fail(f func(r *http.Request) int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = f
}

// Objects returns the stored objects by key.
func (s *Server) Objects() map[string]Object {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.objects)
}

// Uploads returns how many multipart uploads are in progress.
func (s *Server) Uploads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.uploads)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	sum := sha256.Sum256(body)
	key, ok := strings.CutPrefix(r.URL.Path, "/"+s.Bucket+"/")
	q := r.URL.Query()
	s.mu.Lock()
	defer s.mu.Unlock()
	refused := 0
	if s.fail != nil {
		refused = s.fail(r)
	}
	switch {
	case err != nil:
		fail(w, http.StatusBadRequest, "IncompleteBody")
	case !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential="):
		fail(w, http.StatusForbidden, "AccessDenied")
	case r.Header.Get("X-Amz-Content-Sha256") != hex.EncodeToString(sum[:]):
		fail(w, http.StatusBadRequest, "XAmzContentSHA256Mismatch")
	case !ok || key == "":
		fail(w, http.StatusNotFound, "NoSuchBucket")
	case refused != 0:
		fail(w, refused, "InternalError")
	case r.Method == http.MethodPost && q.Has("uploads"):
		id := rand.Text()
		s.uploads[id] = &upload{map[int][]byte{}, r.Header.Clone()}
		answer(w, struct {
			XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
			UploadID string   `xml:"UploadId"`
		}{UploadID: id})
	case r.Method == http.MethodPut && q.Has("uploadId"):
		u, found := s.uploads[q.Get("uploadId")]
		n, _ := strconv.Atoi(q.Get("partNumber"))
		if !found || n < 1 {
			fail(w, http.StatusNotFound, "NoSuchUpload")
			return
		}
		u.parts[n] = body
		w.Header().Set("ETag", fmt.Sprintf("%q", hex.EncodeToString(sum[:16])))
	case r.Method == http.MethodPost && q.Has("uploadId"):
		u, found := s.uploads[q.Get("uploadId")]
		var done struct {
			Parts []struct {
				Number int    `xml:"PartNumber"`
				ETag   string `xml:"ETag"`
			} `xml:"Part"`
		}
		if !found || xml.Unmarshal(body, &done) != nil || len(done.Parts) != len(u.parts) {
			fail(w, http.StatusBadRequest, "InvalidPart")
			return
		}
		var data []byte
		for i, p := range done.Parts {
			part := u.parts[p.Number]
			etag := sha256.Sum256(part)
			if p.Number != i+1 || p.ETag != fmt.Sprintf("%q", hex.EncodeToString(etag[:16])) || len(part) < 5<<20 && i < len(done.Parts)-1 {
				fail(w, http.StatusBadRequest, "InvalidPart")
				return
			}
			data = append(data, part...)
		}
		delete(s.uploads, q.Get("uploadId"))
		s.objects[key] = Object{data, u.header}
		answer(w, struct {
			XMLName xml.Name `xml:"CompleteMultipartUploadResult"`
			Key     string   `xml:"Key"`
		}{Key: key})
	case r.Method == http.MethodDelete && q.Has("uploadId"):
		delete(s.uploads, q.Get("uploadId"))
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPut:
		s.objects[key] = Object{body, r.Header.Clone()}
	case r.Method == http.MethodGet:
		o, found := s.objects[key]
		if !found {
			fail(w, http.StatusNotFound, "NoSuchKey")
			return
		}
		_, _ = w.Write(o.Data)
	case r.Method == http.MethodDelete:
		delete(s.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		fail(w, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

// Keys returns the keys of the stored objects, sorted.
func (s *Server) Keys() []string { return slices.Sorted(maps.Keys(s.Objects())) }

func answer(w http.ResponseWriter, v any) {
	data, _ := xml.Marshal(v)
	_, _ = w.Write(data)
}

func fail(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	answer(w, struct {
		XMLName xml.Name `xml:"Error"`
		Code    string   `xml:"Code"`
		Message string   `xml:"Message"`
	}{Code: code, Message: "The fake storage refused the request."})
}
