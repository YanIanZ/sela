package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
)

var (
	errDuplicateKey = errors.New("duplicate key")
	errFieldName    = errors.New("unknown or case-variant key")
	errTrailingData = errors.New("trailing data")
	errTooDeep      = errors.New("nesting too deep")
)

// maxDepth bounds nesting; request bodies are at most three levels deep.
const maxDepth = 32

var unmarshalerType = reflect.TypeFor[json.Unmarshaler]()

// strictJSON checks that raw is exactly one JSON value whose objects have no
// duplicate keys, and whose keys match the json names of t's struct fields
// exactly. encoding/json matches field names case-insensitively and lets the
// last duplicate win, so {"adultAttested":false,"adultAttested":true} or
// {"AdultAttested":true} would otherwise be accepted.
func strictJSON(raw []byte, t reflect.Type) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := walkValue(dec, t, 0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errTrailingData
	}
	return nil
}

// walkValue consumes one value. t is the Go type it will be decoded into;
// nil (or any type the walker does not model) checks duplicates only.
func walkValue(dec *json.Decoder, t reflect.Type, depth int) error {
	if depth > maxDepth {
		return errTooDeep
	}
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t != nil && reflect.PointerTo(t).Implements(unmarshalerType) {
		t = nil
	}
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch tok {
	case json.Delim('{'):
		var fields map[string]reflect.Type
		var elem reflect.Type
		if t != nil {
			switch t.Kind() {
			case reflect.Struct:
				fields = jsonFields(t)
			case reflect.Map:
				elem = t.Elem()
			}
		}
		seen := map[string]bool{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return err
			}
			key, _ := kt.(string)
			if seen[key] {
				return errDuplicateKey
			}
			seen[key] = true
			next := elem
			if fields != nil {
				ft, ok := fields[key]
				if !ok {
					return errFieldName
				}
				next = ft
			}
			if err := walkValue(dec, next, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	case json.Delim('['):
		var elem reflect.Type
		if t != nil && (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
			elem = t.Elem()
		}
		for dec.More() {
			if err := walkValue(dec, elem, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	return nil
}

var fieldCache sync.Map // reflect.Type -> map[string]reflect.Type

// jsonFields maps the exact json names of a struct's fields (embedded
// struct fields promoted, direct fields first) to their types.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	if v, ok := fieldCache.Load(t); ok {
		return v.(map[string]reflect.Type)
	}
	out := map[string]reflect.Type{}
	addFields(t, out, 0)
	fieldCache.Store(t, out)
	return out
}

func addFields(t reflect.Type, out map[string]reflect.Type, depth int) {
	var embedded []reflect.Type
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				embedded = append(embedded, ft)
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		if _, ok := out[name]; !ok {
			out[name] = f.Type
		}
	}
	if depth < maxDepth {
		for _, ft := range embedded {
			addFields(ft, out, depth+1)
		}
	}
}
