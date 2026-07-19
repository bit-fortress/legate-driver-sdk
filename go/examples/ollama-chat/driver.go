package main

import (
	"bytes"
	"encoding/json"
	"io"

	sdk "github.com/bootun/legate-driver-sdk/go/driver"
	chat "github.com/bootun/legate-driver-sdk/go/protocol/openai/chatcompletions/v20260718"
)

const (
	primaryEndpoint = "primary"
	apiKeySlot      = "api_key"
	boundNone       = "ollama-chat-v2:none"
	boundBearer     = "ollama-chat-v2:bearer"
)

type ollamaChatBinder struct{}
type ollamaChatHandler struct{}

type driverConfig struct {
	Authentication string `json:"authentication"`
}

func (ollamaChatBinder) BindText(request sdk.BindRequest) ([]byte, *sdk.DriverError) {
	config, failure := decodeConfig(request.ConfigJSON)
	if failure != nil {
		return nil, failure
	}
	if !contains(request.EndpointRefs, primaryEndpoint) {
		return nil, invalidConfig("/endpointRefs", "missing_primary", "primary endpoint is required")
	}
	if config.Authentication == "bearer" {
		if !configured(request.CredentialSlots, apiKeySlot) {
			return nil, invalidConfig("/credentialSlots/api_key", "required", "api_key must be configured for bearer authentication")
		}
		return []byte(boundBearer), nil
	}
	return []byte(boundNone), nil
}

func (ollamaChatHandler) OpenAttempt(input chat.OpenAttemptInput) (*chat.OpenAttemptResult, *sdk.DriverError) {
	auth := &sdk.AuthPlan{Kind: sdk.AuthKindNone}
	switch string(input.BoundState) {
	case boundNone:
	case boundBearer:
		auth = &sdk.AuthPlan{Kind: sdk.AuthKindBearer, CredentialSlot: apiKeySlot}
	default:
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidInvocation}
	}
	if input.Mode != sdk.TextModeBuffered && input.Mode != sdk.TextModeSSE {
		return nil, &sdk.DriverError{Code: sdk.ErrorUnsupportedInvocationMode}
	}
	if input.SelectedUpstreamModel == "" || input.ResponseID == "" {
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidInvocation}
	}

	attempt := &ollamaChatAttempt{
		mode:              input.Mode,
		requestModel:      input.Request.Model,
		responseID:        input.ResponseID,
		includeUsageChunk: streamIncludesUsage(input.Request.ExtraFields),
	}
	body, problem := convertOpenAIRequest(input.Request, input.SelectedUpstreamModel, input.Mode)
	if problem != nil {
		response, failure := invalidRequestResponse(*problem)
		if failure != nil {
			return nil, failure
		}
		return &chat.OpenAttemptResult{Attempt: attempt, Response: response}, nil
	}
	return &chat.OpenAttemptResult{Attempt: attempt, Request: &sdk.RequestPlan{
		EndpointRef:  primaryEndpoint,
		Method:       "POST",
		RelativePath: "/api/chat",
		Body:         &sdk.MessageBody{MediaType: sdk.MediaTypeJSON, Payload: body},
		Auth:         auth,
	}}, nil
}

func decodeConfig(raw []byte) (driverConfig, *sdk.DriverError) {
	if !jsonObject(raw) {
		return driverConfig{}, invalidConfig("", "invalid_json", "config must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var config driverConfig
	if decoder.Decode(&config) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return driverConfig{}, invalidConfig("", "invalid_json", "config must contain one JSON object with supported fields")
	}
	if config.Authentication == "" {
		config.Authentication = "none"
	}
	if config.Authentication != "none" && config.Authentication != "bearer" {
		return driverConfig{}, invalidConfig("/authentication", "unsupported", "authentication must be none or bearer")
	}
	return config, nil
}

func invalidRequestResponse(problem requestProblem) (*chat.ClientResponse, *sdk.DriverError) {
	body, err := json.Marshal(openAIErrorEnvelope{Error: openAIError{
		Message: problem.message,
		Type:    "invalid_request_error",
		Param:   problem.pointer,
		Code:    problem.code,
	}})
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	decoded, err := chat.DecodeError(body)
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	response := chat.Response(400, nil, decoded).
		WithOutcome(sdk.CallerError(problem.code)).
		WithUsage(sdk.UnavailableUsage())
	return &response, nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func configured(slots []sdk.CredentialSlotDescriptor, wanted string) bool {
	for _, slot := range slots {
		if slot.Name == wanted {
			return slot.Configured
		}
	}
	return false
}

func invalidConfig(pointer, code, message string) *sdk.DriverError {
	return &sdk.DriverError{
		Code:   sdk.ErrorInvalidConfig,
		Issues: []sdk.FieldIssue{{Pointer: pointer, Code: code, Message: message}},
	}
}

func malformedUpstream() *sdk.DriverError {
	return &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
}

func jsonObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}' && json.Valid(trimmed)
}

func decodeObject(raw []byte) (map[string]json.RawMessage, bool) {
	if !jsonObject(raw) {
		return nil, false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, false
	}
	return object, true
}

func requiredString(fields map[string]json.RawMessage, name, pointer string) (string, *requestProblem) {
	raw, exists := fields[name]
	if !exists {
		return "", problem(pointer, "required", "field is required")
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", problem(pointer, "invalid_type", "field must be a string")
	}
	return value, nil
}

func decimal(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}
