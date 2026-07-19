package main

import (
	"encoding/json"
	"os"
	"testing"

	sdk "github.com/bootun/legate-driver-sdk/go/driver"
	chat "github.com/bootun/legate-driver-sdk/go/protocol/openai/chatcompletions/v20260718"
)

func TestOpenBuildsRequestAtAttemptOpen(t *testing.T) {
	result, failure := dispatcher.OpenTextAttempt(sdk.TextAttemptOpenRequest{BoundState: []byte("openai-chat-compatible-v1"), Invocation: &sdk.TextInvocation{
		Mode: sdk.TextModeBuffered, SelectedUpstreamModel: "upstream-model", ResponseID: "resp_1",
		Request: &sdk.ProtocolPayload{ProtocolContract: sdk.ProtocolContractOpenAIChatCompletions20260718, MediaType: sdk.MediaTypeJSON, JSON: []byte(`{"model":"public-group","messages":[{"role":"user","content":"hi"}]}`)},
	}})
	if failure != nil || result == nil || result.Request == nil {
		t.Fatalf("open = %#v, %#v", result, failure)
	}
	var body map[string]any
	if err := json.Unmarshal(result.Request.Body.Payload, &body); err != nil {
		t.Fatal(err)
	}
	if body["model"] != "upstream-model" || body["stream"] != false {
		t.Fatalf("request body = %#v", body)
	}
}

func TestBufferedPreservesProtocolBody(t *testing.T) {
	attempt := &openAIChatAttempt{mode: sdk.TextModeBuffered}
	payload := []byte(`{"id":"chatcmpl_1","object":"chat.completion","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}`)
	result, failure := attempt.TransformBuffered(sdk.BufferedUpstreamResponse{Head: &sdk.UpstreamResponseHead{StatusCode: 200}, Body: &sdk.MessageBody{MediaType: sdk.MediaTypeJSON, Payload: payload}})
	body, ok := result.Body.(chat.BufferedSuccessResponse)
	if failure != nil || !ok || string(body.JSON) != string(payload) || result.Usage.Status != sdk.UsageStatusFinal {
		t.Fatalf("transform = %#v, %#v", result, failure)
	}
}

func TestManifestContractsMatchTypedHandlerRegistrations(t *testing.T) {
	var manifest struct {
		Text struct {
			ProtocolContracts []string `json:"protocolContracts"`
		} `json:"text"`
	}
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	want := dispatcher.ProtocolContracts()
	if len(manifest.Text.ProtocolContracts) != len(want) || manifest.Text.ProtocolContracts[0] != want[0] {
		t.Fatalf("manifest contracts = %v registrations = %v", manifest.Text.ProtocolContracts, want)
	}
}
