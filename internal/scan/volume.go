//go:build darwin

package scan

import "syscall"

// Volume reports free space for the filesystem holding path.
type Volume struct {
	Free  int64 // bytes available to this user
	Total int64
}

// VolumeAt reads the current free space. macsweep calls it before and after a
// deletion and reports the difference, because a predicted total is never
// quite the truth on APFS: clones share blocks, and local snapshots pin the
// bytes of anything deleted until they thin out. The predicted number is
// advertised as "up to", the measured one as the result.
func VolumeAt(path string) (Volume, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Volume{}, err
	}
	bs := int64(st.Bsize)
	return Volume{
		Free:  int64(st.Bavail) * bs,
		Total: int64(st.Blocks) * bs,
	}, nil
}
