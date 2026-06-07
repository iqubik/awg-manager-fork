package query

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDecodeRCMap(t *testing.T) {
	t.Run("empty array accepted", func(t *testing.T) {
		var dst map[string]any
		if err := decodeRCMap([]byte(`[]`), &dst); err != nil {
			t.Fatalf("decodeRCMap() error = %v", err)
		}
		if dst != nil {
			t.Fatalf("dst = %#v, want nil", dst)
		}
	})

	t.Run("map decoded", func(t *testing.T) {
		var dst map[string]any
		if err := decodeRCMap([]byte(`{"a":{"x":1}}`), &dst); err != nil {
			t.Fatalf("decodeRCMap() error = %v", err)
		}
		if _, ok := dst["a"]; !ok {
			t.Fatalf("decoded map missing key a: %#v", dst)
		}
	})

	t.Run("populated array rejected", func(t *testing.T) {
		var dst map[string]any
		if err := decodeRCMap([]byte(`[{"a":1}]`), &dst); err == nil {
			t.Fatal("decodeRCMap() error = nil, want error")
		}
	})
}

func TestFakeGetterGetRawFallsBackToJSON(t *testing.T) {
	fg := NewFakeGetter()
	fg.SetJSON("/x", `{"ok":true}`)

	raw, err := fg.GetRaw(context.Background(), "/x")
	if err != nil {
		t.Fatalf("GetRaw() error = %v", err)
	}
	if string(raw) != `{"ok":true}` {
		t.Fatalf("GetRaw() = %q", string(raw))
	}
	if fg.Calls("/x") != 1 {
		t.Fatalf("Calls(/x) = %d, want 1", fg.Calls("/x"))
	}
}

func TestFakeGetterGetAndGetRawNoResponse(t *testing.T) {
	fg := NewFakeGetter()

	var dst map[string]any
	if err := fg.Get(context.Background(), "/missing", &dst); err == nil {
		t.Fatal("Get() error = nil, want error")
	}
	if _, err := fg.GetRaw(context.Background(), "/missing"); err == nil {
		t.Fatal("GetRaw() error = nil, want error")
	}
}

func TestFakeGetterPostHandler(t *testing.T) {
	fg := NewFakeGetter()
	fg.SetPostHandler(func(payload any) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	})

	got, err := fg.Post(context.Background(), map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("Post() = %s", string(got))
	}
}

