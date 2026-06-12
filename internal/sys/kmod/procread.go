package kmod

import (
	"io"
	"os"
)

// ReadProc reads a procfs file in a single read(2) call with a buffer
// larger than the kernel side can produce.
//
// os.ReadFile is unsafe for awg_proxy procfs files: it starts with a
// 512-byte buffer (procfs Stat reports size 0) and continues reading at
// offset 512, but awg_proxy.ko < 1.1.11 proc_list_read returned EOF for
// any read at offset > 0 — the list was silently truncated to the first
// 512 bytes once 7+ slots / grown traffic counters pushed the output
// past that boundary, and tunnel start failed with "endpoint not found
// in proxy list" (issue #362). One big read returns the kernel's full
// list buffer (4096 bytes max) on both old and fixed module versions.
func ReadProc(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}
