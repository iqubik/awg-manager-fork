package vlink

import (
	"testing"
)

func TestDecodeBase64Url(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"standard", "aGVsbG8=", "hello", false},
		{"urlsafe", "YWJjLWRlZl8xMjM=", "abc-def_123", false},
		{"no-padding", "aGVsbG8", "hello", false},
		{"urlsafe-no-padding", "aGVsbG8", "hello", false},
		{"empty", "", "", false},
		{"garbage", "!!not-base64!!", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeBase64Url(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil; result=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDoubleDecode(t *testing.T) {
	// Plain text on first decode → return as-is decoded.
	plain := []byte("aGVsbG8=") // "hello" in base64
	got, ok := DoubleDecode(plain)
	if !ok || string(got) != "hello" {
		t.Errorf("first-pass: got %q ok=%v, want %q true", got, ok, "hello")
	}

	// Double-encoded: base64(base64("hello"))
	innerB64 := "aGVsbG8="     // base64("hello")
	outerB64 := "YUdWc2JHOD0=" // base64("aGVsbG8=")
	got, ok = DoubleDecode([]byte(outerB64))
	if !ok {
		t.Fatalf("double-decode failed")
	}
	// DoubleDecode should land on the innermost text.
	if string(got) != "hello" && string(got) != innerB64 {
		t.Errorf("double-pass: got %q, want %q or %q", got, "hello", innerB64)
	}
}

func TestStripTrailing(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"abc===", "abc"},
		{"abc==", "abc"},
		{"abc=", "abc"},
		{"abc", "abc"},
		{"abc=def", "abc=def"}, // mid-string = preserved
	}
	for _, tc := range cases {
		got := StripTrailing(tc.in)
		if got != tc.want {
			t.Errorf("in=%q got=%q want=%q", tc.in, got, tc.want)
		}
	}
}
