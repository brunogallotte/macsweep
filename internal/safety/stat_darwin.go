//go:build darwin

package safety

import (
	"io/fs"
	"syscall"
)

const sfRestricted = 0x00080000

func owner(fi fs.FileInfo) *uint32 {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return &st.Uid
}

// restricted reports whether System Integrity Protection holds this file.
// Such a file cannot be removed even by root, so offering it as reclaimable
// would only produce a failure the user cannot act on.
func restricted(fi fs.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return st.Flags&sfRestricted != 0
}
