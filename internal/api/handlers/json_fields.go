package handlers

// This file holds tri-state JSON field wrappers for PATCH-style request
// bodies. Plain Go pointers can only distinguish two states (absent/nil vs.
// value); these wrappers track three:
//
//   - absent:  the key was not present in the JSON body (Set = false) —
//     the caller should leave the current value unchanged
//   - null:    the key was present with an explicit JSON null (Set = true)
//   - value:   the key was present with a concrete value (Set = true)
//
// How null is decoded depends on the wrapper: optionalIntSliceField keeps a
// nil slice (e.g. "all libraries"), optionalStringSliceField normalizes null
// to an empty slice.

import (
	"bytes"
	"encoding/json"
)

// optionalIntSliceField distinguishes an absent JSON field from an explicit
// null or array value. Absent = don't update; null = nil slice ("all
// libraries"); [] = empty slice ("none").
type optionalIntSliceField struct {
	Set   bool
	Value []int
}

func (f *optionalIntSliceField) UnmarshalJSON(data []byte) error {
	f.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.Value = nil
		return nil
	}
	return json.Unmarshal(data, &f.Value)
}

// Ptr returns nil when the field was absent, otherwise a pointer to the
// decoded slice (which may itself be nil for JSON null).
func (f optionalIntSliceField) Ptr() *[]int {
	if !f.Set {
		return nil
	}
	value := f.Value
	return &value
}

// optionalStringSliceField distinguishes an absent JSON field from an
// explicit null or array value. JSON null decodes to an empty slice.
type optionalStringSliceField struct {
	Set   bool
	Value []string
}

func (f *optionalStringSliceField) UnmarshalJSON(data []byte) error {
	f.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.Value = []string{}
		return nil
	}
	return json.Unmarshal(data, &f.Value)
}
