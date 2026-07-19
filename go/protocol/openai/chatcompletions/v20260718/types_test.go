package v20260718

import (
	"encoding/json"
	"testing"
)

func TestRequestPreservesUnknownFields(t *testing.T) {
	request, err := DecodeRequest([]byte(`{"model":"group","messages":[],"stream":true,"future_option":{"x":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Model = "upstream"
	encoded, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["model"]) != `"upstream"` || string(fields["future_option"]) != `{"x":1}` {
		t.Fatalf("encoded = %s", encoded)
	}
}

func TestDecodersRejectDuplicateKeysAtEveryDepth(t *testing.T) {
	for _, input := range []string{
		`{"model":"one","model":"two","messages":[]}`,
		`{"model":"one","messages":[{"role":"user","role":"assistant"}]}`,
	} {
		if _, err := DecodeRequest([]byte(input)); err == nil {
			t.Fatalf("DecodeRequest(%s) accepted duplicate key", input)
		}
	}
}
