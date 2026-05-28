package orchestrator

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SaveDraft writes the slot's JSON to pending/<filename> atomically.
// Idempotent overwrite. Does NOT schedule reload — staging is intentionally
// inert until ApplyDraft is called.
//
// Returns ErrUnknownSlot if the slot is not registered.
func (o *Orchestrator) SaveDraft(slot Slot, jsonBytes []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	meta, ok := o.slots[slot]
	if !ok {
		return ErrUnknownSlot
	}
	if err := writeAtomic(o.pendingPath(meta), jsonBytes); err != nil {
		return fmt.Errorf("SaveDraft %s: %w", slot, err)
	}
	return nil
}

// LoadEffective returns the bytes of the "most relevant" copy of a slot
// for UI consumers. Priority chain:
//
//   1. pending/<filename>  — user's in-flight edits (SaveDraft target)
//   2. active/<filename>   — applied config, slot enabled
//   3. disabled/<filename> — saved config, slot disabled
//   4. (nil, nil)          — slot never configured
//
// Including disabled/ makes "engine off" mean "inactive but editable":
// UI handlers (ListRules etc.) keep showing the user's rules so they can
// be reviewed and re-enabled without re-entering. Returns ErrUnknownSlot
// when the slot is not registered.
func (o *Orchestrator) LoadEffective(slot Slot) ([]byte, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	meta, ok := o.slots[slot]
	if !ok {
		return nil, ErrUnknownSlot
	}
	data, err := readIfExists(o.pendingPath(meta))
	if err != nil {
		return nil, fmt.Errorf("LoadEffective pending %s: %w", slot, err)
	}
	if data != nil {
		return data, nil
	}
	data, err = readIfExists(o.activePath(meta))
	if err != nil {
		return nil, fmt.Errorf("LoadEffective active %s: %w", slot, err)
	}
	if data != nil {
		return data, nil
	}
	data, err = readIfExists(o.disabledPath(meta))
	if err != nil {
		return nil, fmt.Errorf("LoadEffective disabled %s: %w", slot, err)
	}
	return data, nil
}

// HasDraft reports whether a pending file exists for the slot.
// Lock-free presence check internally — acquires only briefly to read
// the slot meta map.
func (o *Orchestrator) HasDraft(slot Slot) bool {
	o.mu.Lock()
	meta, ok := o.slots[slot]
	o.mu.Unlock()
	if !ok {
		return false
	}
	_, err := os.Stat(o.pendingPath(meta))
	return err == nil
}

// DiscardDraft removes the pending file for the slot. Idempotent: no
// error if pending was absent. Returns ErrUnknownSlot if slot not
// registered.
func (o *Orchestrator) DiscardDraft(slot Slot) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	meta, ok := o.slots[slot]
	if !ok {
		return ErrUnknownSlot
	}
	return removeIfExists(o.pendingPath(meta))
}

// DraftInfo returns metadata about the pending file for the slot.
// HasDraft false implies DraftedAt zero. ErrUnknownSlot is silently
// translated to a zero DraftInfo (the caller is typically a status
// handler that should not panic on misconfiguration).
func (o *Orchestrator) DraftInfo(slot Slot) DraftInfo {
	o.mu.Lock()
	meta, ok := o.slots[slot]
	o.mu.Unlock()
	if !ok {
		return DraftInfo{}
	}
	st, err := os.Stat(o.pendingPath(meta))
	if err != nil {
		return DraftInfo{}
	}
	return DraftInfo{HasDraft: true, DraftedAt: st.ModTime()}
}

// ApplyDraft validates the pending draft and, if it passes, atomically
// renames pending → active and arms a reload. The validation pipeline
// is: (1) cross-slot validateDraftLocked → (2) sing-box check on a
// tmpdir snapshot mirroring all enabled slots with the target swapped
// for the draft → (3) os.Rename(pending, active).
//
// Returns (ValidationResult, nil) when the logical check fails — the
// caller should surface the errors to the user; pending is preserved
// for further editing.
//
// Returns (ZeroResult, ErrNoDraft) when there is no pending file.
//
// Returns (ZeroResult, wrapped error) on FS or external-check failures.
// In all error paths, pending is preserved and active is unchanged.
func (o *Orchestrator) ApplyDraft(slot Slot) (ValidationResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	meta, ok := o.slots[slot]
	if !ok {
		return ValidationResult{}, ErrUnknownSlot
	}
	pendingPath := o.pendingPath(meta)
	draftBytes, err := os.ReadFile(pendingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ValidationResult{}, ErrNoDraft
		}
		return ValidationResult{}, fmt.Errorf("ApplyDraft read pending %s: %w", slot, err)
	}

	// Cross-slot validation against draft.
	if res := o.validateDraftLocked(slot, draftBytes); !res.Ok() {
		return res, nil
	}

	// sing-box check against tmpdir snapshot.
	if o.validator != nil {
		tmpdir, err := os.MkdirTemp(o.configDir, ".apply-check-*")
		if err != nil {
			return ValidationResult{}, fmt.Errorf("ApplyDraft tmpdir: %w", err)
		}
		defer os.RemoveAll(tmpdir)

		for s, en := range o.enabled {
			if !en {
				continue
			}
			m, ok := o.slots[s]
			if !ok {
				continue
			}
			dst := filepath.Join(tmpdir, m.Filename)
			if s == slot {
				if err := writeAtomic(dst, draftBytes); err != nil {
					return ValidationResult{}, fmt.Errorf("ApplyDraft snapshot draft: %w", err)
				}
				continue
			}
			if err := copyFile(o.activePath(m), dst); err != nil {
				if os.IsNotExist(err) {
					continue // slot enabled but file not yet written — fine
				}
				return ValidationResult{}, fmt.Errorf("ApplyDraft snapshot %s: %w", s, err)
			}
		}

		if err := o.validator.Validate(context.Background(), tmpdir); err != nil {
			return ValidationResult{}, fmt.Errorf("sing-box check: %w", err)
		}
	}

	// Atomic commit.
	if err := os.Rename(pendingPath, o.activePath(meta)); err != nil {
		return ValidationResult{}, fmt.Errorf("ApplyDraft rename: %w", err)
	}
	o.dirty = true
	o.scheduleReload()
	return ValidationResult{}, nil
}

// CheckMerged runs the full validation pipeline (cross-slot logical
// check + `sing-box check` over a tmpdir snapshot of every enabled slot
// with the target slot overlaid by jsonBytes) WITHOUT writing anything.
//
// Used by callers that need iterative validation — e.g. the subscription
// adapter dropping one bad outbound at a time after parsing the
// `outbound[INDEX]` from sing-box's error — and don't want a partial
// write between iterations.
//
// Unlike ApplyDraft, the target slot is included in the snapshot
// regardless of its current o.enabled value: "validate as if applied",
// not "validate against currently-active siblings only".
func (o *Orchestrator) CheckMerged(slot Slot, jsonBytes []byte) (ValidationResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.checkMergedLocked(slot, jsonBytes)
}

// checkMergedLocked is the shared body for SaveAndValidate / CheckMerged.
// Caller MUST hold o.mu.
func (o *Orchestrator) checkMergedLocked(slot Slot, jsonBytes []byte) (ValidationResult, error) {
	meta, ok := o.slots[slot]
	if !ok {
		return ValidationResult{}, ErrUnknownSlot
	}

	if res := o.validateDraftLocked(slot, jsonBytes); !res.Ok() {
		return res, nil
	}

	if o.validator == nil {
		return ValidationResult{}, nil
	}

	tmpdir, err := os.MkdirTemp(o.configDir, ".save-check-*")
	if err != nil {
		return ValidationResult{}, fmt.Errorf("CheckMerged tmpdir: %w", err)
	}
	defer os.RemoveAll(tmpdir)

	for s, en := range o.enabled {
		if !en || s == slot {
			continue
		}
		m, ok := o.slots[s]
		if !ok {
			continue
		}
		dst := filepath.Join(tmpdir, m.Filename)
		if err := copyFile(o.activePath(m), dst); err != nil && !os.IsNotExist(err) {
			return ValidationResult{}, fmt.Errorf("CheckMerged snapshot %s: %w", s, err)
		}
	}
	if err := writeAtomic(filepath.Join(tmpdir, meta.Filename), jsonBytes); err != nil {
		return ValidationResult{}, fmt.Errorf("CheckMerged write target: %w", err)
	}

	if err := o.validator.Validate(context.Background(), tmpdir); err != nil {
		// Sing-box check failure is reported as a ValidationError so callers
		// can use res.Error() / iterate without distinguishing infra vs
		// content errors at the type level.
		return ValidationResult{Errors: []ValidationError{{
			Slot:    slot,
			Kind:    "sing-box check",
			Message: err.Error(),
		}}}, nil
	}
	return ValidationResult{}, nil
}

// SaveAndValidate atomically writes jsonBytes to the slot's active path
// after running CheckMerged. On success: arms a debounced reload. On
// validation failure: returns a non-empty ValidationResult and leaves the
// active file untouched. On infra failure (mkdir/write): returns wrapped
// error, active still untouched.
//
// One-shot convenience for callers that don't need iterative correction;
// see CheckMerged for the read-only variant used by the subscription
// filter loop (issue #221).
func (o *Orchestrator) SaveAndValidate(slot Slot, jsonBytes []byte) (ValidationResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	res, err := o.checkMergedLocked(slot, jsonBytes)
	if err != nil {
		return ValidationResult{}, err
	}
	if !res.Ok() {
		return res, nil
	}

	meta := o.slots[slot] // validated by checkMergedLocked
	if err := writeAtomic(o.activePath(meta), jsonBytes); err != nil {
		return ValidationResult{}, fmt.Errorf("SaveAndValidate write active: %w", err)
	}
	o.dirty = true
	o.scheduleReload()
	return ValidationResult{}, nil
}

// ValidateDraft is the lock-acquiring public form of validateDraftLocked.
// Used by handlers that want to surface "would this draft apply" in the
// UI without committing.
func (o *Orchestrator) ValidateDraft(slot Slot, draftBytes []byte) ValidationResult {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.validateDraftLocked(slot, draftBytes)
}

// copyFile copies src → dst. dst is overwritten if it exists. Used by
// ApplyDraft to assemble the tmpdir snapshot.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cErr := out.Close(); err == nil {
		err = cErr
	}
	return err
}
