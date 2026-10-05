//go:build unix

package app

import (
	"os"
	"strconv"
	"syscall"
)

// ownerOf returns the ID of the user who owns a file.
func ownerOf(info os.FileInfo) (string, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", false
	}
	return strconv.FormatUint(uint64(st.Uid), 10), true
}
