package main

import (
	"encoding/json"
	"os"
	"testing"

	sdk "github.com/bootun/legate-driver-sdk/go/driver"
	chat "github.com/bootun/legate-driver-sdk/go/protocol/openai/chatcompletions/v20260718"
)

func TestBindSupportsLocalAndBearerAuthentication(t *testing.T) {
	binder := ollamaChatBinder{}
	local, failure := binder.BindText(sdk.BindRequest{ConfigJSON: []byte(`{}`), EndpointRefs: []string{primaryEndpoint}})
	if failure != nil || string(local) != boundNone {
		t.Fatalf("local BindText() = %q, %#v", local, failure)
	}
	bearer, failure := binder.BindText(sdk.BindRequest{
		ConfigJSON: []byte(`{"authentication":"bearer"}`), EndpointRefs: []string{primaryEndpoint},
		CredentialSlots: []sdk.CredentialSlotDescriptor{{Name: apiKeySlot, Configured: true}},
	})
	if failure != nil || string(bearer) != boundBearer {
		t.Fatalf("bearer BindText() = %q, %#v", bearer, failure)
	}
}

func TestBindRejectsInvalidConfiguration(t *testing.T) {
	for index, request := range []sdk.BindRequest{
		{ConfigJSON: []byte(`null`), EndpointRefs: []string{primaryEndpoint}},
		{ConfigJSON: []byte(`{"unknown":true}`), EndpointRefs: []string{primaryEndpoint}},
		{ConfigJSON: []byte(`{"authentication":"basic"}`), EndpointRefs: []string{primaryEndpoint}},
		{ConfigJSON: []byte(`{}`)},
		{ConfigJSON: []byte(`{"authentication":"bearer"}`), EndpointRefs: []string{primaryEndpoint}},
	} {
		if _, failure := (ollamaChatBinder{}).BindText(request); failure == nil || failure.Code != sdk.ErrorInvalidConfig {
			t.Fatalf("case %d: failure = %#v", index, failure)
		}
	}
}

func TestOpenAttemptConvertsOpenAIRequest(t *testing.T) {
	result := openAttempt(t, sdk.TextModeBuffered, boundBearer, `{
		"model":"chat-group",
		"messages":[
			{"role":"developer","content":"Be concise."},
			{"role":"user","content":[
				{"type":"text","text":"Describe this."},
				{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}
			]}
		],
		"temperature":0.2,
		"max_completion_tokens":42,
		"response_format":{"type":"json_object"},
		"stream":false
	}`)
	if result.Request == nil || result.Response != nil || result.Request.RelativePath != "/api/chat" || result.Request.Auth.Kind != sdk.AuthKindBearer {
		t.Fatalf("open result = %#v", result)
	}
	fields := objectFromBody(t, result.Request.Body.Payload)
	assertJSONString(t, fields["model"], "qwen3:8b")
	assertJSONBool(t, fields["stream"], false)
	assertJSONString(t, fields["format"], "json")
	var options map[string]json.RawMessage
	mustUnmarshal(t, fields["options"], &options)
	if string(options["temperature"]) != "0.2" || string(options["num_predict"]) != "42" {
		t.Fatalf("options = %s", fields["options"])
	}
	var messages []ollamaMessage
	mustUnmarshal(t, fields["messages"], &messages)
	if len(messages) != 2 || messages[0].Role != "system" || messages[1].Content != "Describe this." || len(messages[1].Images) != 1 {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestOpenAttemptEnablesNativeNDJSONForSSE(t *testing.T) {
	result := openAttempt(t, sdk.TextModeSSE, boundNone, `{"model":"chat-group","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	fields := objectFromBody(t, result.Request.Body.Payload)
	assertJSONBool(t, fields["stream"], true)
}

func TestOpenAttemptReturnsProtocolErrorForUnsupportedField(t *testing.T) {
	result := openAttempt(t, sdk.TextModeBuffered, boundNone, `{"model":"chat-group","messages":[],"n":2}`)
	if result.Response == nil || result.Request != nil || result.Response.StatusCode != 400 || result.Response.Usage == nil || result.Response.Outcome == nil {
		t.Fatalf("open result = %#v", result)
	}
}

func TestTransformBufferedConvertsSuccessAndUsage(t *testing.T) {
	opened := openAttempt(t, sdk.TextModeBuffered, boundNone, `{"model":"chat-group","messages":[]}`)
	client, failure := opened.Attempt.TransformBuffered(sdk.BufferedUpstreamResponse{
		Head: &sdk.UpstreamResponseHead{StatusCode: 200},
		Body: &sdk.MessageBody{MediaType: sdk.MediaTypeJSON, Payload: []byte(`{
			"model":"qwen3:8b","created_at":"2026-07-15T12:00:00Z",
			"message":{"role":"assistant","content":"sunny","thinking":"checked"},
			"done":true,"done_reason":"stop","prompt_eval_count":11,"eval_count":7
		}`)},
	})
	if failure != nil || client == nil || client.Usage == nil || client.Usage.Status != sdk.UsageStatusFinal {
		t.Fatalf("TransformBuffered() = %#v, %#v", client, failure)
	}
	body := client.Body.(chat.BufferedSuccessResponse)
	var completion openAICompletion
	mustUnmarshal(t, body.JSON, &completion)
	if completion.ID != "chatcmpl-legate-1" || completion.Model != "chat-group" || completion.Usage.TotalTokens != 18 || completion.Choices[0].Message.ReasoningContent != "checked" {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestTransformBufferedMapsModelNotFound(t *testing.T) {
	opened := openAttempt(t, sdk.TextModeBuffered, boundNone, `{"model":"chat-group","messages":[]}`)
	client, failure := opened.Attempt.TransformBuffered(sdk.BufferedUpstreamResponse{
		Head: &sdk.UpstreamResponseHead{StatusCode: 404},
		Body: &sdk.MessageBody{MediaType: sdk.MediaTypeJSON, Payload: []byte(`{"error":"model not found"}`)},
	})
	if failure != nil || client.Outcome == nil || client.Outcome.Class != sdk.SemanticOutcomeClassMappingError || client.Usage.Status != sdk.UsageStatusUnavailable {
		t.Fatalf("TransformBuffered() = %#v, %#v", client, failure)
	}
}

func TestTransformNDJSONStreamProducesOpenAIChunksAndFinalUsage(t *testing.T) {
	opened := openAttempt(t, sdk.TextModeSSE, boundNone, `{"model":"chat-group","messages":[],"stream":true,"stream_options":{"include_usage":true}}`)
	attempt := opened.Attempt
	if _, failure := attempt.OpenStream(sdk.UpstreamResponseHead{StatusCode: 200}); failure != nil {
		t.Fatal(failure)
	}
	first, failure := attempt.TransformStreamEvent(sdk.UpstreamSSEEvent{Data: []byte(`{
		"model":"qwen3:8b","created_at":"2026-07-15T12:00:00Z",
		"message":{"role":"assistant","content":"hello"},"done":false
	}`)})
	if failure != nil || len(first.Events) != 1 {
		t.Fatalf("first event = %#v, %#v", first, failure)
	}
	final, failure := attempt.TransformStreamEvent(sdk.UpstreamSSEEvent{Data: []byte(`{
		"model":"qwen3:8b","created_at":"2026-07-15T12:00:01Z",
		"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop",
		"prompt_eval_count":3,"eval_count":1
	}`)})
	if failure != nil || len(final.Events) != 2 || final.Outcome == nil || final.Outcome.Class != sdk.SemanticOutcomeClassSuccess || final.Usage == nil || final.Usage.Status != sdk.UsageStatusFinal {
		t.Fatalf("final event = %#v, %#v", final, failure)
	}
	var terminal openAIChunk
	mustUnmarshal(t, final.Events[0].JSON, &terminal)
	if terminal.Choices[0].FinishReason == nil || *terminal.Choices[0].FinishReason != "stop" {
		t.Fatalf("terminal chunk = %#v", terminal)
	}
	var usage openAIChunk
	mustUnmarshal(t, final.Events[1].JSON, &usage)
	if usage.Usage == nil || usage.Usage.TotalTokens != 4 || len(usage.Choices) != 0 {
		t.Fatalf("usage chunk = %#v", usage)
	}
	if _, failure := attempt.FinishStream(); failure != nil {
		t.Fatal(failure)
	}
}

func TestManifestContractsMatchTypedRegistration(t *testing.T) {
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
		t.Fatalf("manifest contracts = %v, registrations = %v", manifest.Text.ProtocolContracts, want)
	}
}

func openAttempt(t *testing.T, mode sdk.TextMode, state, body string) *chat.OpenAttemptResult {
	t.Helper()
	request, err := chat.DecodeRequest([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	result, failure := (ollamaChatHandler{}).OpenAttempt(chat.OpenAttemptInput{
		BoundState: []byte(state), Mode: mode, Request: request,
		SelectedUpstreamModel: "qwen3:8b", ResponseID: "chatcmpl-legate-1",
	})
	if failure != nil || result == nil {
		t.Fatalf("OpenAttempt() = %#v, %#v", result, failure)
	}
	return result
}

func objectFromBody(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	mustUnmarshal(t, body, &fields)
	return fields
}

func mustUnmarshal(t *testing.T, raw []byte, output any) {
	t.Helper()
	if err := json.Unmarshal(raw, output); err != nil {
		t.Fatalf("json.Unmarshal(%s): %v", raw, err)
	}
}

func assertJSONString(t *testing.T, raw json.RawMessage, wanted string) {
	t.Helper()
	var got string
	mustUnmarshal(t, raw, &got)
	if got != wanted {
		t.Fatalf("JSON string = %q, want %q", got, wanted)
	}
}

func assertJSONBool(t *testing.T, raw json.RawMessage, wanted bool) {
	t.Helper()
	var got bool
	mustUnmarshal(t, raw, &got)
	if got != wanted {
		t.Fatalf("JSON bool = %v, want %v", got, wanted)
	}
}
