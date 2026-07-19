package main

import (
	"bytes"
	"encoding/json"

	sdk "github.com/bootun/legate-driver-sdk/go/driver"
	chat "github.com/bootun/legate-driver-sdk/go/protocol/openai/chatcompletions/v20260718"
)

type openAIChatBinder struct{}
type openAIChatHandler struct{}

func (openAIChatBinder) BindText(request sdk.BindRequest) ([]byte, *sdk.DriverError) {
	if !sdk.IsEmptyJSONObject(request.ConfigJSON) {
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidConfig}
	}
	if !contains(request.EndpointRefs, "primary") || !configured(request.CredentialSlots, "api_key") {
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidConfig}
	}
	return []byte("openai-chat-compatible-v1"), nil
}

func (openAIChatHandler) OpenAttempt(input chat.OpenAttemptInput) (*chat.OpenAttemptResult, *sdk.DriverError) {
	if string(input.BoundState) != "openai-chat-compatible-v1" {
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidInvocation}
	}
	input.Request.Model = input.SelectedUpstreamModel
	input.Request.Stream = input.Mode == sdk.TextModeSSE
	if input.Mode == sdk.TextModeSSE {
		input.Request.ExtraFields["stream_options"] = json.RawMessage(`{"include_usage":true}`)
	}
	encoded, err := input.Request.Encode()
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	attempt := &openAIChatAttempt{mode: input.Mode}
	return &chat.OpenAttemptResult{Attempt: attempt, Request: &sdk.RequestPlan{
		EndpointRef: "primary", Method: "POST", RelativePath: "/v1/chat/completions",
		Body: &sdk.MessageBody{MediaType: sdk.MediaTypeJSON, Payload: encoded},
		Auth: &sdk.AuthPlan{Kind: sdk.AuthKindBearer, CredentialSlot: "api_key"},
	}}, nil
}

type openAIChatAttempt struct {
	mode     sdk.TextMode
	finished bool
	closed   bool
}

func (a *openAIChatAttempt) TransformBuffered(upstream sdk.BufferedUpstreamResponse) (*chat.ClientResponse, *sdk.DriverError) {
	if a.closed || a.mode != sdk.TextModeBuffered {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	if upstream.Head == nil || upstream.Body == nil || !json.Valid(upstream.Body.Payload) {
		return nil, &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
	}
	var body chat.ResponseBody
	var err error
	if upstream.Head.StatusCode >= 200 && upstream.Head.StatusCode < 400 {
		body, err = chat.DecodeBufferedSuccess(upstream.Body.Payload)
	} else {
		body, err = chat.DecodeError(upstream.Body.Payload)
	}
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
	}
	response := chat.Response(upstream.Head.StatusCode, publicHeaders(upstream.Head.Headers), body).
		WithUsage(usageFromJSON(upstream.Body.Payload))
	return &response, nil
}

func (a *openAIChatAttempt) OpenStream(upstream sdk.UpstreamResponseHead) (*sdk.TextSSEOpenSuccess, *sdk.DriverError) {
	if a.closed || a.mode != sdk.TextModeSSE {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	return &sdk.TextSSEOpenSuccess{StatusCode: upstream.StatusCode, Headers: publicHeaders(upstream.Headers)}, nil
}

func (a *openAIChatAttempt) TransformStreamEvent(upstream sdk.UpstreamSSEEvent) (*chat.StreamEventResult, *sdk.DriverError) {
	if a.closed || a.mode != sdk.TextModeSSE || a.finished {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	if bytes.Equal(bytes.TrimSpace(upstream.Data), []byte("[DONE]")) {
		a.finished = true
		outcome := sdk.Success()
		usage := sdk.UnavailableUsage()
		return &chat.StreamEventResult{Outcome: &outcome, Usage: &usage}, nil
	}
	event, err := chat.DecodeStreamEvent("chat.completion.chunk", upstream.Data)
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
	}
	return &chat.StreamEventResult{Events: []chat.StreamEvent{event}}, nil
}

func (a *openAIChatAttempt) FinishStream() (*chat.StreamFinishResult, *sdk.DriverError) {
	if a.closed {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	if !a.finished {
		return nil, &sdk.DriverError{Code: sdk.ErrorUpstreamStreamTruncated}
	}
	return &chat.StreamFinishResult{}, nil
}

func (a *openAIChatAttempt) Close() *sdk.DriverError { a.closed = true; return nil }

func usageFromJSON(payload []byte) sdk.UsageReport {
	var value struct {
		Usage *struct {
			PromptTokens     *int64 `json:"prompt_tokens"`
			CompletionTokens *int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(payload, &value) == nil && value.Usage != nil && value.Usage.PromptTokens != nil && value.Usage.CompletionTokens != nil {
		return sdk.UsageReport{Status: sdk.UsageStatusFinal, InputTokens: value.Usage.PromptTokens,
			OutputTokens: value.Usage.CompletionTokens, Provenance: sdk.UsageProvenanceUpstreamReported}
	}
	return sdk.UnavailableUsage()
}

func publicHeaders(headers []sdk.NameValues) []sdk.NameValues {
	for _, header := range headers {
		if header.Name == "retry-after" || header.Name == "Retry-After" {
			return []sdk.NameValues{{Name: "retry-after", Values: append([]string(nil), header.Values...)}}
		}
	}
	return nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func configured(values []sdk.CredentialSlotDescriptor, want string) bool {
	for _, value := range values {
		if value.Name == want {
			return value.Configured
		}
	}
	return false
}
