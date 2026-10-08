package datastore

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// archive returns a ZIP archive with files of the given names and contents, deflated if
// deflate is set, otherwise stored.
func archive(t *testing.T, deflate bool, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < len(files); i += 2 {
		method := zip.Store
		if deflate {
			method = zip.Deflate
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: files[i], Method: method})
		must(t, err)
		_, err = io.WriteString(w, files[i+1])
		must(t, err)
	}
	must(t, zw.Close())
	return buf.Bytes()
}

// An uploaded archive may only hold a <database>.sql for each database, besides folders and
// what macOS adds, and must be complete and unpack to a sensible size.
func TestSQLFiles(t *testing.T) {
	read := func(data []byte) (map[string]*zip.File, error) {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		must(t, err)
		return sqlFiles(zr, int64(len(data)))
	}
	files, err := read(archive(t, true, "luckperms.sql", "CREATE TABLE a;", "litebans.sql", "", "__MACOSX/._luckperms.sql", "junk", "folder/", ""))
	must(t, err)
	if names := slices.Sorted(maps.Keys(files)); !slices.Equal(names, []string{"litebans", "luckperms"}) {
		t.Fatalf("files %v", names)
	}

	damaged := archive(t, false, "luckperms.sql", "CREATE TABLE a;")
	damaged[bytes.Index(damaged, []byte("CREATE"))] = 'X'
	many := []string{}
	for i := range maxEntries + 1 {
		many = append(many, fmt.Sprintf("db%d.sql", i), "")
	}
	tooManyDatabases := []string{}
	for i := range noryxv1.MaxDatabases + 1 {
		tooManyDatabases = append(tooManyDatabases, fmt.Sprintf("db%d.sql", i), "")
	}
	for name, data := range map[string][]byte{
		"a path":          archive(t, true, "../luckperms.sql", ""),
		"a folder":        archive(t, true, "dump/luckperms.sql", ""),
		"upper case":      archive(t, true, "LuckPerms.sql", ""),
		"a reserved name": archive(t, true, "mysql.sql", ""),
		"another file":    archive(t, true, "luckperms.sql", "", "readme.txt", ""),
		"twice":           archive(t, true, "luckperms.sql", "", "luckperms.sql", ""),
		"nothing":         archive(t, true, "folder/", ""),
		"damaged":         damaged,
		"too many files":  archive(t, false, many...),
		"too many":        archive(t, false, tooManyDatabases...),
		"far more inside": archive(t, true, "luckperms.sql", strings.Repeat("\x00", 101<<20)),
	} {
		if _, err := read(data); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}
}
