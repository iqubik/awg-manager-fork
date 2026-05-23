//go:build !windows

package routerinfo

import "syscall"

func statfsUsage(path string) (used, total int64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil || st.Blocks <= 0 {
		return 0, 0, false
	}

	blockSize := int64(st.Bsize)
	total = int64(st.Blocks) * blockSize
	used = int64(st.Blocks-st.Bfree) * blockSize
	if total <= 0 || used < 0 {
		return 0, 0, false
	}
	return used, total, true
}
