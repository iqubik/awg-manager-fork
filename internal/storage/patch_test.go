package storage

import (
	"testing"
)

type tHappyDst struct {
	A bool
	B string
	C int
	D struct {
		Field string
	}
}

type tHappyPatch struct {
	A *bool
	B *string
	C *int
	D *struct {
		Field string
	}
}

func ptrBool(v bool) *bool    { return &v }
func ptrStr(v string) *string { return &v }
func ptrInt(v int) *int       { return &v }

func TestApplyPatch_HappyPath_BoolFields(t *testing.T) {
	dst := tHappyDst{A: false}
	patch := tHappyPatch{A: ptrBool(true)}
	ApplyPatch(&dst, &patch)
	if dst.A != true {
		t.Errorf("A = %v, want true", dst.A)
	}
}

func TestApplyPatch_HappyPath_StringFields(t *testing.T) {
	dst := tHappyDst{B: "old"}
	patch := tHappyPatch{B: ptrStr("new")}
	ApplyPatch(&dst, &patch)
	if dst.B != "new" {
		t.Errorf("B = %q, want new", dst.B)
	}
}

func TestApplyPatch_HappyPath_StructValueField(t *testing.T) {
	dst := tHappyDst{}
	dst.D.Field = "old"
	patch := tHappyPatch{D: &struct{ Field string }{Field: "x"}}
	ApplyPatch(&dst, &patch)
	if dst.D.Field != "x" {
		t.Errorf("D.Field = %q, want x", dst.D.Field)
	}
}

func TestApplyPatch_NilPointerSkipped(t *testing.T) {
	dst := tHappyDst{A: false, B: "keep"}
	patch := tHappyPatch{A: ptrBool(true), B: nil}
	ApplyPatch(&dst, &patch)
	if dst.A != true {
		t.Errorf("A = %v, want true", dst.A)
	}
	if dst.B != "keep" {
		t.Errorf("B = %q, want keep (nil patch field must not overwrite)", dst.B)
	}
}

func TestApplyPatch_AppliesAllFieldsInOnePass(t *testing.T) {
	dst := tHappyDst{A: false, B: "old", C: 0}
	dst.D.Field = "old"
	patch := tHappyPatch{
		A: ptrBool(true),
		B: ptrStr("new"),
		C: ptrInt(42),
		D: &struct{ Field string }{Field: "newD"},
	}
	ApplyPatch(&dst, &patch)
	if dst.A != true || dst.B != "new" || dst.C != 42 || dst.D.Field != "newD" {
		t.Errorf("multi-field apply failed: %+v", dst)
	}
}
