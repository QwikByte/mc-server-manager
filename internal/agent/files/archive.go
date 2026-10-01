package files

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// stored lists extensions of files that are compressed already; deflating them again
// costs time for nothing, which matters for worlds of several gigabytes.
var stored = map[string]bool{".mca": true, ".jar": true, ".zip": true, ".gz": true, ".png": true, ".jpg": true, ".ogg": true}

// writeZip writes the directory as a ZIP archive. Symbolic links and other special
// files are left out.
func writeZip(w io.Writer, dir *os.Root) error {
	zw := zip.NewWriter(w)
	err := fs.WalkDir(dir.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || name == "." || (!d.IsDir() && !d.Type().IsRegular()) {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		if d.IsDir() {
			header.Name += "/"
		} else if !stored[strings.ToLower(path.Ext(name))] {
			header.Method = zip.Deflate
		}
		fw, err := zw.CreateHeader(header)
		if err != nil || d.IsDir() {
			return err
		}
		f, err := dir.Open(name)
		if err != nil {
			return err
		}
		_, err = io.Copy(fw, f)
		return errors.Join(err, f.Close())
	})
	return errors.Join(err, zw.Close())
}
