package httpapi

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

type strictInner struct {
	Mode string `json:"mode"`
}

type strictBase struct {
	Shared string `json:"shared"`
	Inner  string `json:"inner"`
}

type strictTarget struct {
	strictBase
	Inner    int                    `json:"inner"` // direct field wins over the embedded one
	Name     string                 `json:"name,omitempty"`
	Plain    string                 // no tag: exact Go name
	Skipped  string                 `json:"-"`
	Identity *strictInner           `json:"identity"`
	List     []strictInner          `json:"list"`
	Labels   map[string]strictInner `json:"labels"`
	Free     any                    `json:"free"`
	Raw      json.RawMessage        `json:"raw"`
	When     time.Time              `json:"when"`
	hidden   string
}

func TestStrictJSONRejectsDuplicateAndCaseVariantKeys(t *testing.T) {
	typ := reflect.TypeOf(&strictTarget{})
	for body, ok := range map[string]bool{
		`{"name":"a","Plain":"b","shared":"c","inner":1}`:                                   true,
		`{"identity":{"mode":"anonymous"},"list":[{"mode":"x"},{"mode":"y"}]}`:              true,
		`{"labels":{"A":{"mode":"x"},"a":{"mode":"y"}}}`:                                    true, // map keys are data
		`{"free":{"Any":{"Case":[1,{"x":2}]}},"raw":{"X":1},"when":"2026-10-26T00:00:00Z"}`: true,
		` {"name":"a"} `:                       true,
		`{"name":"a","name":"b"}`:              false,
		`{"NAME":"a"}`:                         false,
		`{"Name":"a"}`:                         false,
		`{"plain":"b"}`:                        false,
		`{"Skipped":"x"}`:                      false,
		`{"hidden":"x"}`:                       false,
		`{"identity":{"MODE":"anonymous"}}`:    false,
		`{"identity":{"mode":"a","mode":"b"}}`: false,
		`{"list":[{"mode":"x"},{"Mode":"y"}]}`: false,
		`{"labels":{"a":{"mode":"x"},"a":{}}}`: false,
		`{"labels":{"a":{"MODE":"x"}}}`:        false,
		`{"free":{"x":1,"x":2}}`:               false,
		`{"raw":{"x":1,"x":2}}`:                false,
		`{"name":"a"}{}`:                       false,
		`{"name":"a"} x`:                       false,
		`{"name":`:                             false,
		``:                                     false,
		`{"free":` + deep(40) + `}`:            false,
	} {
		err := strictJSON([]byte(body), typ)
		if (err == nil) != ok {
			t.Errorf("strictJSON(%s) = %v, want ok=%t", body, err, ok)
		}
	}
}

func deep(n int) string {
	s := "1"
	for i := 0; i < n; i++ {
		s = "[" + s + "]"
	}
	return s
}
