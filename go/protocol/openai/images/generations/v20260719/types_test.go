package v20260719

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRequestStrictJSONAndUnknownFieldRoundTrip(t *testing.T) {
	if _, err := DecodeRequest([]byte(`{"model":"group","prompt":"x","nested":{"a":1,"a":2}}`)); err == nil {
		t.Fatal("nested duplicate key was accepted")
	}
	if _, err := DecodeRequest([]byte(`{"model":"group","prompt":"x"} {}`)); err == nil {
		t.Fatal("multiple JSON values were accepted")
	}
	request, err := DecodeRequest([]byte(`{"model":"group","prompt":"draw","future":{"x":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Model = "vendor-image-v2"
	encoded, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(encoded, &fields) != nil || !bytes.Equal(fields["future"], []byte(`{"x":1}`)) ||
		string(fields["model"]) != `"vendor-image-v2"` {
		t.Fatalf("round trip = %s", encoded)
	}
}

func TestTypedResponsesPreserveUnknownFields(t *testing.T) {
	response, err := DecodeSuccessResponse([]byte(`{"created":1,"data":[{"b64_json":"eA==","future":true}],"future_top":7}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := response.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(encoded, &fields) != nil || string(fields["future_top"]) != "7" {
		t.Fatalf("response round trip = %s", encoded)
	}
}
