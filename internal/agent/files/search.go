package files

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/fileset"
	"github.com/QwikByte/noryx/internal/agent/secrets"
)

// Limits of searches, so that one ends soon and its answer stays small.
const (
	maxQuery    = 200
	maxMatches  = 500
	searchTime  = 10 * time.Second
	maxSearched = 4 << 20 // the largest file searched
	maxLine     = 240     // of a line in a match
)

// SearchFiles searches the text files of a folder and those in it for a text, regardless of
// case, as the panel shows them: files that only hold secrets and the temporary files of the
// agent are left out, others show secrets as the placeholder, and links aren't followed.
func (s *Service) SearchFiles(ctx context.Context, req *noryxv1.SearchFilesRequest) (*noryxv1.SearchFilesResponse, error) {
	query := strings.ToLower(req.GetQuery())
	if strings.TrimSpace(query) == "" || utf8.RuneCountInString(query) > maxQuery || strings.ContainsFunc(query, unicode.IsControl) {
		return nil, status.Errorf(codes.InvalidArgument, "Enter up to %d characters to search for.", maxQuery)
	}
	dir, name, err := s.open(ctx, req.GetServerId(), req.GetPath())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	if err := dir.Direct(name); err != nil {
		return nil, toStatus(err)
	}
	if info, err := dir.Lstat(name); err != nil {
		return nil, toStatus(err)
	} else if !info.IsDir() {
		return nil, status.Error(codes.InvalidArgument, "Choose a folder to search.")
	}
	ctx, cancel := context.WithTimeout(ctx, searchTime)
	defer cancel()
	hidden := fileset.Watch(dir) // a file set may write files with secrets meanwhile
	res := &noryxv1.SearchFilesResponse{}
	err = fs.WalkDir(dir.FS(), filepath.ToSlash(name), func(p string, e fs.DirEntry, err error) error {
		switch {
		case ctx.Err() != nil || len(res.Matches) >= maxMatches:
			res.Truncated = true
			return fs.SkipAll
		case err != nil:
			return nil //nolint:nilerr // e.g. a folder the agent can't read, which is skipped
		case e.IsDir() && datadir.IsTemp(e.Name()):
			return fs.SkipDir
		case datadir.IsTemp(e.Name()) || !e.Type().IsRegular(): // a file being written, e.g. with secrets filled in
			return nil
		}
		info, err := e.Info()
		switch {
		case err != nil:
			return nil //nolint:nilerr // e.g. a file that is gone
		case info.Size() > maxSearched:
			res.TooLarge++
			return nil
		}
		matches, searched := searchFile(dir, filepath.FromSlash(p), query, hidden)
		if searched {
			res.Files++
		}
		res.Matches = append(res.Matches, matches...)
		return nil
	})
	if err != nil {
		return nil, toStatus(err)
	}
	if len(res.Matches) > maxMatches {
		res.Matches, res.Truncated = res.Matches[:maxMatches], true
	}
	return res, nil
}

// searchFile returns the lines of a text file that hold query, in lower case, as the panel
// shows them, and whether it searched the file: not one with secrets only, nor one that isn't
// UTF-8 text.
func searchFile(dir *datadir.Dir, name, query string, hidden func() secrets.Files) ([]*noryxv1.SearchMatch, bool) {
	f, err := dir.Open(name)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	if hidden().Hidden(name) { // checked once the file is open, see ReadFile
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSearched))
	if err != nil || bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return nil, false
	}
	data = secrets.Redact(name, data)
	if !bytes.Contains(bytes.ToLower(data), []byte(query)) {
		return nil, true
	}
	var matches []*noryxv1.SearchMatch
	for i, line := range strings.Split(string(data), "\n") {
		if at := strings.Index(strings.ToLower(line), query); at >= 0 {
			matches = append(matches, &noryxv1.SearchMatch{Path: filepath.ToSlash(name), Line: int64(i + 1), Text: excerpt(line, at)})
		}
	}
	return matches, true
}

// excerpt shortens a long line to the part around the match at the given byte.
func excerpt(line string, at int) string {
	line = strings.TrimRight(line, "\r")
	if len(line) <= maxLine {
		return line
	}
	start := max(min(at-maxLine/3, len(line)-maxLine), 0)
	for start > 0 && !utf8.RuneStart(line[start]) {
		start--
	}
	end := min(start+maxLine, len(line))
	for end < len(line) && !utf8.RuneStart(line[end]) {
		end--
	}
	text := line[start:end]
	if start > 0 {
		text = "…" + text
	}
	if end < len(line) {
		text += "…"
	}
	return text
}
