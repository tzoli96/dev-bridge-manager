// backend/internal/models/json_list_test.go
package models

import (
	"encoding/json"
	"testing"
)

type jlItem struct {
	Label string  `json:"label"`
	N     float64 `json:"n"`
}

func TestJSONListValueNilIsEmptyArray(t *testing.T) {
	var l JSONList[jlItem]
	v, err := l.Value()
	if err != nil || v != "[]" {
		t.Fatalf("got %v, %v", v, err)
	}
}

func TestJSONListValueMarshalsItems(t *testing.T) {
	v, err := JSONList[jlItem]{{Label: "a", N: 1.5}}.Value()
	if err != nil || v != `[{"label":"a","n":1.5}]` {
		t.Fatalf("got %v, %v", v, err)
	}
}

func TestJSONListScanAcceptsBytesAndString(t *testing.T) {
	for _, src := range []any{[]byte(`[{"label":"x","n":2}]`), `[{"label":"x","n":2}]`} {
		var l JSONList[jlItem]
		if err := l.Scan(src); err != nil {
			t.Fatalf("scan %T: %v", src, err)
		}
		if len(l) != 1 || l[0].Label != "x" || l[0].N != 2 {
			t.Fatalf("scan %T: got %+v", src, l)
		}
	}
}

func TestJSONListScanNilIsEmptyNotNil(t *testing.T) {
	l := JSONList[jlItem]{{Label: "stale"}}
	if err := l.Scan(nil); err != nil {
		t.Fatal(err)
	}
	if l == nil || len(l) != 0 {
		t.Fatalf("got %#v", l)
	}
}

func TestJSONListScanRejectsBadInput(t *testing.T) {
	var l JSONList[jlItem]
	if err := l.Scan(`{"not":"a list"}`); err == nil {
		t.Fatal("expected error for a JSON object")
	}
	if err := l.Scan(42); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestJSONListMarshalJSONNilIsEmptyArray(t *testing.T) {
	b, err := json.Marshal(struct {
		L JSONList[jlItem] `json:"l"`
	}{})
	if err != nil || string(b) != `{"l":[]}` {
		t.Fatalf("got %s, %v", b, err)
	}
}

func TestJSONListRoundTrip(t *testing.T) {
	in := JSONList[jlItem]{{Label: "a", N: 1}, {Label: "b", N: 2}}
	v, err := in.Value()
	if err != nil {
		t.Fatal(err)
	}
	var out JSONList[jlItem]
	if err := out.Scan(v); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[1].Label != "b" || out[1].N != 2 {
		t.Fatalf("got %+v", out)
	}
}

func TestJSONListScanJSONNullIsEmptyNonNil(t *testing.T) {
	var l JSONList[jlItem]
	if err := l.Scan([]byte("null")); err != nil {
		t.Fatal(err)
	}
	if l == nil || len(l) != 0 {
		t.Fatalf("got %#v", l)
	}
}
