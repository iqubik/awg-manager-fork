package storage

import "testing"

func TestAppendUnique(t *testing.T) {
	got, added := appendUnique([]string{"a", "b"}, "c")
	if !added || len(got) != 3 || got[2] != "c" {
		t.Fatalf("appendUnique add failed: %#v %v", got, added)
	}
	got, added = appendUnique([]string{"a", "b"}, "a")
	if added || len(got) != 2 {
		t.Fatalf("appendUnique duplicate failed: %#v %v", got, added)
	}
}

func TestFilterOut(t *testing.T) {
	got := filterOut([]string{"a", "b", "a", "c"}, "a")
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Fatalf("filterOut failed: %#v", got)
	}
}

func TestContains(t *testing.T) {
	if !contains([]string{"x", "y"}, "x") {
		t.Fatal("contains should find x")
	}
	if contains([]string{"x", "y"}, "z") {
		t.Fatal("contains should not find z")
	}
}

