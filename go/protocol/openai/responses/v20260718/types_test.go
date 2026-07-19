package v20260718

import "testing"

func TestRequestPreservesUnknownFields(t *testing.T) {
	request, err := DecodeRequest([]byte(`{"model":"group","input":"hello","future_option":1}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 || string(request.ExtraFields["future_option"]) != "1" {
		t.Fatalf("request = %#v", request)
	}
}

func TestDecoderRejectsDuplicateKeys(t *testing.T) {
	if _, err := DecodeRequest([]byte(`{"model":"group","input":{"x":1,"x":2}}`)); err == nil {
		t.Fatal("DecodeRequest accepted duplicate key")
	}
}
