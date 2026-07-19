package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	sdk "github.com/bootun/legate-driver-sdk/go/driver"
	chat "github.com/bootun/legate-driver-sdk/go/protocol/openai/chatcompletions/v20260718"
)

type ollamaChatResponse struct {
	Model           string                `json:"model"`
	CreatedAt       string                `json:"created_at"`
	Message         ollamaResponseMessage `json:"message"`
	Done            *bool                 `json:"done"`
	DoneReason      string                `json:"done_reason"`
	PromptEvalCount *int64                `json:"prompt_eval_count"`
	EvalCount       *int64                `json:"eval_count"`
	Logprobs        json.RawMessage       `json:"logprobs"`
	Error           string                `json:"error"`
}

type ollamaResponseMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Thinking  string           `json:"thinking"`
	ToolCalls []ollamaToolCall `json:"tool_calls"`
}

type openAICompletion struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

type openAIChoice struct {
	Index        int             `json:"index"`
	Message      openAIMessage   `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Logprobs     *openAILogprobs `json:"logprobs,omitempty"`
}

type openAIMessage struct {
	Role             string           `json:"role"`
	Content          string           `json:"content"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolCallDelta struct {
	Index    int                `json:"index"`
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAILogprobs struct {
	Content json.RawMessage `json:"content"`
}

type openAIUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type openAIErrorEnvelope struct {
	Error openAIError `json:"error"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code"`
}

type openAIChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []openAIChunkChoice `json:"choices"`
	Usage   *openAIUsage        `json:"usage,omitempty"`
}

type openAIChunkChoice struct {
	Index        int             `json:"index"`
	Delta        openAIDelta     `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
	Logprobs     *openAILogprobs `json:"logprobs,omitempty"`
}

type openAIDelta struct {
	Role             string                `json:"role,omitempty"`
	Content          string                `json:"content,omitempty"`
	ReasoningContent string                `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIToolCallDelta `json:"tool_calls,omitempty"`
}

type ollamaChatAttempt struct {
	mode              sdk.TextMode
	requestModel      string
	responseID        string
	created           int64
	sentRole          bool
	sawToolCalls      bool
	nextToolIndex     int
	includeUsageChunk bool
	finished          bool
	closed            bool
}

func (a *ollamaChatAttempt) TransformBuffered(upstream sdk.BufferedUpstreamResponse) (*chat.ClientResponse, *sdk.DriverError) {
	if a == nil || a.closed || upstream.Head == nil || upstream.Body == nil {
		return nil, malformedUpstream()
	}
	if upstream.Head.StatusCode < 200 || upstream.Head.StatusCode >= 300 {
		return transformOllamaError(*upstream.Head, upstream.Body.Payload)
	}
	if a.mode != sdk.TextModeBuffered {
		return nil, malformedUpstream()
	}
	response, failure := decodeOllamaResponse(upstream.Body.Payload)
	if failure != nil || response.Done == nil || !*response.Done || response.Model == "" || response.Message.Role != "assistant" {
		return nil, malformedUpstream()
	}
	created, ok := parseCreated(response.CreatedAt)
	if !ok {
		return nil, malformedUpstream()
	}
	usage, openAIUsage, ok := finalUsage(response.PromptEvalCount, response.EvalCount)
	if !ok {
		return nil, malformedUpstream()
	}
	toolCalls, failure := convertOutboundToolCalls(response.Message.ToolCalls, a.responseID, 0)
	if failure != nil {
		return nil, failure
	}
	choice := openAIChoice{
		Index: 0,
		Message: openAIMessage{
			Role:             "assistant",
			Content:          response.Message.Content,
			ReasoningContent: response.Message.Thinking,
			ToolCalls:        toolCalls,
		},
		FinishReason: finishReason(response.DoneReason, len(toolCalls) > 0),
	}
	if logprobs, ok := convertLogprobs(response.Logprobs); ok {
		choice.Logprobs = logprobs
	} else if len(bytes.TrimSpace(response.Logprobs)) > 0 && !bytes.Equal(bytes.TrimSpace(response.Logprobs), []byte("null")) {
		return nil, malformedUpstream()
	}
	body, err := json.Marshal(openAICompletion{
		ID: a.responseID, Object: "chat.completion", Created: created, Model: a.requestModel,
		Choices: []openAIChoice{choice}, Usage: openAIUsage,
	})
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	decoded, err := chat.DecodeBufferedSuccess(body)
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidProtocolResponse}
	}
	client := chat.Response(upstream.Head.StatusCode, publicHeaders(upstream.Head.Headers), decoded).WithUsage(usage)
	return &client, nil
}

func (a *ollamaChatAttempt) OpenStream(upstream sdk.UpstreamResponseHead) (*sdk.TextSSEOpenSuccess, *sdk.DriverError) {
	if a == nil || a.closed || a.mode != sdk.TextModeSSE || upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
		return nil, malformedUpstream()
	}
	return &sdk.TextSSEOpenSuccess{StatusCode: upstream.StatusCode, Headers: publicHeaders(upstream.Headers)}, nil
}

func (a *ollamaChatAttempt) TransformStreamEvent(upstream sdk.UpstreamSSEEvent) (*chat.StreamEventResult, *sdk.DriverError) {
	if a == nil || a.closed || a.mode != sdk.TextModeSSE || a.finished {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	response, failure := decodeOllamaResponse(upstream.Data)
	if failure != nil {
		return nil, failure
	}
	if response.Error != "" {
		a.finished = true
		outcome := sdk.EndpointError("ollama_stream_error")
		usage := sdk.UnavailableUsage()
		return &chat.StreamEventResult{Outcome: &outcome, Usage: &usage}, nil
	}
	if response.Done == nil || response.Model == "" || response.Message.Role != "assistant" {
		return nil, malformedUpstream()
	}
	created, ok := parseCreated(response.CreatedAt)
	if !ok {
		return nil, malformedUpstream()
	}
	a.created = created

	events := make([]chat.StreamEvent, 0, 3)
	delta, failure := a.streamDelta(response.Message)
	if failure != nil {
		return nil, failure
	}
	if !a.sentRole || delta.Content != "" || delta.ReasoningContent != "" || len(delta.ToolCalls) > 0 {
		if !a.sentRole {
			delta.Role = "assistant"
			a.sentRole = true
		}
		chunk, failure := a.chunkEvent(delta, nil, response.Logprobs, nil)
		if failure != nil {
			return nil, failure
		}
		events = append(events, chunk)
	}
	if !*response.Done {
		return &chat.StreamEventResult{Events: events}, nil
	}

	finish := finishReason(response.DoneReason, a.sawToolCalls)
	terminal, failure := a.chunkEvent(openAIDelta{}, &finish, nil, nil)
	if failure != nil {
		return nil, failure
	}
	events = append(events, terminal)
	usage, openAIUsage, usageOK := finalUsage(response.PromptEvalCount, response.EvalCount)
	if !usageOK {
		usage = sdk.UnavailableUsage()
	} else if a.includeUsageChunk {
		usageChunk, failure := a.usageEvent(openAIUsage)
		if failure != nil {
			return nil, failure
		}
		events = append(events, usageChunk)
	}
	a.finished = true
	outcome := sdk.Success()
	return &chat.StreamEventResult{Events: events, Outcome: &outcome, Usage: &usage}, nil
}

func (a *ollamaChatAttempt) FinishStream() (*chat.StreamFinishResult, *sdk.DriverError) {
	if a == nil || a.closed || a.mode != sdk.TextModeSSE {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	if !a.finished {
		return nil, &sdk.DriverError{Code: sdk.ErrorUpstreamStreamTruncated}
	}
	return &chat.StreamFinishResult{}, nil
}

func (a *ollamaChatAttempt) Close() *sdk.DriverError {
	if a == nil || a.closed {
		return &sdk.DriverError{Code: sdk.ErrorInvalidAttemptState}
	}
	a.closed = true
	return nil
}

func (a *ollamaChatAttempt) streamDelta(message ollamaResponseMessage) (openAIDelta, *sdk.DriverError) {
	delta := openAIDelta{Content: message.Content, ReasoningContent: message.Thinking}
	for _, call := range message.ToolCalls {
		if call.Function.Name == "" || !jsonObject(call.Function.Arguments) {
			return openAIDelta{}, malformedUpstream()
		}
		index := a.nextToolIndex
		a.nextToolIndex++
		a.sawToolCalls = true
		delta.ToolCalls = append(delta.ToolCalls, openAIToolCallDelta{
			Index: index,
			ID:    toolCallID(a.responseID, index),
			Type:  "function",
			Function: openAIToolFunction{
				Name: call.Function.Name, Arguments: string(call.Function.Arguments),
			},
		})
	}
	return delta, nil
}

func (a *ollamaChatAttempt) chunkEvent(delta openAIDelta, finish *string, rawLogprobs json.RawMessage, usage *openAIUsage) (chat.StreamEvent, *sdk.DriverError) {
	choice := openAIChunkChoice{Index: 0, Delta: delta, FinishReason: finish}
	if logprobs, ok := convertLogprobs(rawLogprobs); ok {
		choice.Logprobs = logprobs
	} else if len(bytes.TrimSpace(rawLogprobs)) > 0 && !bytes.Equal(bytes.TrimSpace(rawLogprobs), []byte("null")) {
		return chat.StreamEvent{}, malformedUpstream()
	}
	body, err := json.Marshal(openAIChunk{
		ID: a.responseID, Object: "chat.completion.chunk", Created: a.created,
		Model: a.requestModel, Choices: []openAIChunkChoice{choice}, Usage: usage,
	})
	if err != nil {
		return chat.StreamEvent{}, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	event, err := chat.DecodeStreamEvent("chat.completion.chunk", body)
	if err != nil {
		return chat.StreamEvent{}, &sdk.DriverError{Code: sdk.ErrorInvalidProtocolResponse}
	}
	return event, nil
}

func (a *ollamaChatAttempt) usageEvent(usage openAIUsage) (chat.StreamEvent, *sdk.DriverError) {
	body, err := json.Marshal(openAIChunk{
		ID: a.responseID, Object: "chat.completion.chunk", Created: a.created,
		Model: a.requestModel, Choices: []openAIChunkChoice{}, Usage: &usage,
	})
	if err != nil {
		return chat.StreamEvent{}, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	event, err := chat.DecodeStreamEvent("chat.completion.chunk", body)
	if err != nil {
		return chat.StreamEvent{}, &sdk.DriverError{Code: sdk.ErrorInvalidProtocolResponse}
	}
	return event, nil
}

func decodeOllamaResponse(payload []byte) (ollamaChatResponse, *sdk.DriverError) {
	if !jsonObject(payload) {
		return ollamaChatResponse{}, malformedUpstream()
	}
	var response ollamaChatResponse
	if json.Unmarshal(payload, &response) != nil {
		return ollamaChatResponse{}, malformedUpstream()
	}
	return response, nil
}

func transformOllamaError(head sdk.UpstreamResponseHead, payload []byte) (*chat.ClientResponse, *sdk.DriverError) {
	fields, ok := decodeObject(payload)
	if !ok {
		return nil, malformedUpstream()
	}
	var vendorMessage string
	if json.Unmarshal(fields["error"], &vendorMessage) != nil || vendorMessage == "" {
		return nil, malformedUpstream()
	}
	code := "ollama_error"
	message := "The upstream Ollama request failed."
	outcome := sdk.EndpointError(code)
	switch head.StatusCode {
	case 400, 413, 422:
		code = "ollama_request_rejected"
		message = "The upstream rejected the request."
		outcome = sdk.CallerError(code)
	case 404:
		code = "ollama_model_not_found"
		message = "The configured Ollama model was not found."
		outcome = sdk.MappingError(code)
	}
	body, err := json.Marshal(openAIErrorEnvelope{Error: openAIError{Message: message, Type: code, Code: code}})
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	decoded, err := chat.DecodeError(body)
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidProtocolResponse}
	}
	response := chat.Response(head.StatusCode, publicHeaders(head.Headers), decoded).
		WithOutcome(outcome).
		WithUsage(sdk.UnavailableUsage())
	return &response, nil
}

func convertOutboundToolCalls(input []ollamaToolCall, responseID string, start int) ([]openAIToolCall, *sdk.DriverError) {
	output := make([]openAIToolCall, 0, len(input))
	for index, call := range input {
		if call.Function.Name == "" || !jsonObject(call.Function.Arguments) {
			return nil, malformedUpstream()
		}
		position := start + index
		output = append(output, openAIToolCall{
			ID: toolCallID(responseID, position), Type: "function",
			Function: openAIToolFunction{Name: call.Function.Name, Arguments: string(call.Function.Arguments)},
		})
	}
	return output, nil
}

func toolCallID(responseID string, index int) string {
	clean := strings.TrimPrefix(responseID, "chatcmpl-")
	return "call_" + clean + "_" + decimal(index)
}

func finalUsage(input, output *int64) (sdk.UsageReport, openAIUsage, bool) {
	if input == nil || output == nil || *input < 0 || *output < 0 || *input > int64(^uint64(0)>>1)-*output {
		return sdk.UsageReport{}, openAIUsage{}, false
	}
	in := *input
	out := *output
	return sdk.UsageReport{
		Status: sdk.UsageStatusFinal, InputTokens: &in, OutputTokens: &out,
		Provenance: sdk.UsageProvenanceUpstreamReported,
	}, openAIUsage{PromptTokens: in, CompletionTokens: out, TotalTokens: in + out}, true
}

func parseCreated(value string) (int64, bool) {
	created, err := time.Parse(time.RFC3339Nano, value)
	return created.Unix(), err == nil
}

func finishReason(reason string, hasToolCalls bool) string {
	if hasToolCalls {
		return "tool_calls"
	}
	if reason == "length" {
		return "length"
	}
	return "stop"
}

func convertLogprobs(raw json.RawMessage) (*openAILogprobs, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false
	}
	if !json.Valid(trimmed) || trimmed[0] != '[' {
		return nil, false
	}
	return &openAILogprobs{Content: append(json.RawMessage(nil), trimmed...)}, true
}

func publicHeaders(headers []sdk.NameValues) []sdk.NameValues {
	for _, header := range headers {
		if strings.EqualFold(header.Name, "retry-after") {
			return []sdk.NameValues{{Name: "retry-after", Values: append([]string(nil), header.Values...)}}
		}
	}
	return nil
}
