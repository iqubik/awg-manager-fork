package routerinfo

import (
	"errors"
	"testing"
)

func TestFetchOPKGStorage_FallbackToOptStatfs(t *testing.T) {
	origGetJSON := rciGetJSONFunc
	origGetRaw := rciGetRawFunc
	origStatfs := statfsUsageFunc
	t.Cleanup(func() {
		rciGetJSONFunc = origGetJSON
		rciGetRawFunc = origGetRaw
		statfsUsageFunc = origStatfs
	})

	rciGetJSONFunc = func(path string, dst any) error {
		return errors.New("rci unavailable")
	}
	rciGetRawFunc = func(path string) ([]byte, error) {
		return nil, errors.New("rci unavailable")
	}
	statfsUsageFunc = func(path string) (used, total int64, ok bool) {
		if path != "/opt" {
			t.Fatalf("unexpected path: %s", path)
		}
		return 32 * 1024 * 1024, 55 * 1024 * 1024, true
	}

	got := fetchOPKGStorage()
	if got != "32 MB / 55 MB" {
		t.Fatalf("unexpected opkg storage: got %q, want %q", got, "32 MB / 55 MB")
	}
}

func TestFetchOPKGStorage_PrefersRCIValue(t *testing.T) {
	origGetJSON := rciGetJSONFunc
	origGetRaw := rciGetRawFunc
	origStatfs := statfsUsageFunc
	t.Cleanup(func() {
		rciGetJSONFunc = origGetJSON
		rciGetRawFunc = origGetRaw
		statfsUsageFunc = origStatfs
	})

	rciGetJSONFunc = func(path string, dst any) error {
		if path != "/show/sc/opkg/disk" {
			return errors.New("unexpected path")
		}
		d, ok := dst.(*rciOpkgDiskWire)
		if !ok {
			return errors.New("unexpected dst type")
		}
		d.Disk = "mydisk"
		return nil
	}
	rciGetRawFunc = func(path string) ([]byte, error) {
		if path != "/ls" {
			return nil, errors.New("unexpected path")
		}
		return []byte(`{"mydisk:":{"free":1048576,"total":2097152}}`), nil
	}
	statfsUsageFunc = func(path string) (used, total int64, ok bool) {
		return 999, 1000, true
	}

	got := fetchOPKGStorage()
	if got != "1 MB / 2 MB" {
		t.Fatalf("unexpected opkg storage: got %q, want %q", got, "1 MB / 2 MB")
	}
}

func TestFetchOPKGStorage_EmptyWhenNoRCIAndNoStatfs(t *testing.T) {
	origGetJSON := rciGetJSONFunc
	origGetRaw := rciGetRawFunc
	origStatfs := statfsUsageFunc
	t.Cleanup(func() {
		rciGetJSONFunc = origGetJSON
		rciGetRawFunc = origGetRaw
		statfsUsageFunc = origStatfs
	})

	rciGetJSONFunc = func(path string, dst any) error {
		return errors.New("rci unavailable")
	}
	rciGetRawFunc = func(path string) ([]byte, error) {
		return nil, errors.New("rci unavailable")
	}
	statfsUsageFunc = func(path string) (used, total int64, ok bool) {
		return 0, 0, false
	}

	got := fetchOPKGStorage()
	if got != "" {
		t.Fatalf("unexpected opkg storage: got %q, want empty", got)
	}
}
