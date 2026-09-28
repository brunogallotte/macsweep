package scan

import (
	"io/fs"
	"syscall"
	"time"
)

// blockSize is the unit st_blocks is reported in on macOS, always 512 bytes
// regardless of the filesystem's own block size.
const blockSize = 512

// Allocated reports how many bytes an entry actually occupies on disk.
//
// It is derived from st_blocks, which is what du reports. The logical size
// lies in two cases that matter a lot on a Mac: sparse files (Docker.raw
// claims tens of GB it does not use) and APFS clones (copies that share
// blocks with the original until one of them is written to).
func Allocated(fi fs.FileInfo) int64 {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fi.Size()
	}
	return st.Blocks * blockSize
}

// Flags from sys/stat.h that change how an entry must be treated.
const (
	// sfDataless marks an iCloud placeholder: the metadata is local but the
	// content is not. Opening one triggers a download, so these are measured
	// and then left strictly alone.
	sfDataless = 0x40000000
	// sfRestricted marks a file protected by System Integrity Protection.
	// Not even root can remove it, so it must never be offered as
	// reclaimable.
	sfRestricted = 0x00080000
)

// fileID identifies an inode on a device. Two directory entries sharing one
// fileID are hard links to the same data, so the bytes must only be counted
// once.
type fileID struct {
	dev int32
	ino uint64
}

type statInfo struct {
	id      fileID
	links   uint16
	uid     uint32
	mode    uint16
	size    int64
	modTime time.Time
	flags   uint32
	ok      bool
}

func (s statInfo) dataless() bool   { return s.flags&sfDataless != 0 }
func (s statInfo) restricted() bool { return s.flags&sfRestricted != 0 }

func inspect(fi fs.FileInfo) statInfo {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return statInfo{size: fi.Size(), modTime: fi.ModTime()}
	}
	return statInfo{
		id:      fileID{dev: st.Dev, ino: st.Ino},
		links:   st.Nlink,
		uid:     st.Uid,
		mode:    st.Mode,
		size:    st.Blocks * blockSize,
		modTime: fi.ModTime(),
		flags:   st.Flags,
		ok:      true,
	}
}
