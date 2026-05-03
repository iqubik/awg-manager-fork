package storage

import (
	"fmt"
	"reflect"
)

// ApplyPatch copies non-nil pointer fields from src into the
// matching-named fields of dst. See contract in
// docs/superpowers/specs/2026-05-03-settings-defense-in-depth-design.md §4.
//
// Generic only for ergonomic call sites; the implementation is fully
// reflective and accepts any pair of struct types.
//
// Panics on programmer errors (nil dst, non-struct dst, non-pointer field
// in src, type mismatch on assignable field) so misuse fails fast at the
// first call rather than silently corrupting state.
func ApplyPatch[Dst any, Src any](dst *Dst, src *Src) {
	if dst == nil {
		panic("ApplyPatch: dst is nil")
	}
	if src == nil {
		return
	}
	dstV := reflect.ValueOf(dst).Elem()
	srcV := reflect.ValueOf(src).Elem()
	if dstV.Kind() != reflect.Struct {
		panic("ApplyPatch: dst is not a struct pointer")
	}
	if srcV.Kind() != reflect.Struct {
		panic("ApplyPatch: src is not a struct pointer")
	}
	srcT := srcV.Type()
	for i := 0; i < srcV.NumField(); i++ {
		srcField := srcV.Field(i)
		srcFieldT := srcT.Field(i)
		if !srcFieldT.IsExported() {
			continue
		}
		if srcField.Kind() != reflect.Pointer {
			panic(fmt.Sprintf("ApplyPatch: field %s in patch type %s is not a pointer (every patch field must be a pointer)", srcFieldT.Name, srcT.Name()))
		}
		if srcField.IsNil() {
			continue
		}
		dstField := dstV.FieldByName(srcFieldT.Name)
		if !dstField.IsValid() {
			continue // src has a field dst doesn't — silently ignore (DTO drift tolerated)
		}
		if !dstField.CanSet() {
			continue
		}
		srcInner := srcField.Elem()
		if dstField.Kind() == reflect.Pointer {
			if srcField.Type() == dstField.Type() {
				dstField.Set(srcField)
				continue
			}
			if srcInner.Type().AssignableTo(dstField.Type().Elem()) {
				dstField.Set(srcField)
				continue
			}
			panic(fmt.Sprintf("ApplyPatch: incompatible pointer types for field %s: dst=%s src=%s", srcFieldT.Name, dstField.Type(), srcField.Type()))
		}
		if !srcInner.Type().AssignableTo(dstField.Type()) {
			panic(fmt.Sprintf("ApplyPatch: incompatible types for field %s: dst=%s src deref=%s", srcFieldT.Name, dstField.Type(), srcInner.Type()))
		}
		dstField.Set(srcInner)
	}
}
