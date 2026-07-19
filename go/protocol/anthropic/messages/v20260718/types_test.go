package v20260718

import "testing"

func TestRequestAndMetadataPreserveUnknownFields(t *testing.T) {
	request, err := DecodeRequest([]byte(`{"model":"group","messages":[],"future_option":true}`))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := DecodeMetadata([]byte(`{"version":"2023-06-01","betas":["tools"],"future_header":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(request.ExtraFields["future_option"]) != "true" || string(metadata.ExtraFields["future_header"]) != `"x"` {
		t.Fatalf("request = %#v metadata = %#v", request, metadata)
	}
}

func TestDecodersRejectDuplicateKeys(t *testing.T) {
	if _, err := DecodeRequest([]byte(`{"model":"group","messages":[],"model":"other"}`)); err == nil {
		t.Fatal("DecodeRequest accepted duplicate key")
	}
	if _, err := DecodeMetadata([]byte(`{"version":"one","version":"two"}`)); err == nil {
		t.Fatal("DecodeMetadata accepted duplicate key")
	}
}
