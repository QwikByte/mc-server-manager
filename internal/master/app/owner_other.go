//go:build !unix

package app

import "os"

// ownerOf doesn't know the owner of files on other systems than Unix, where the master only
// runs for development.
func ownerOf(os.FileInfo) (string, bool) { return "", false }
