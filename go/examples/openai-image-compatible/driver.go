package main

import (
	"strings"

	sdk "github.com/bootun/legate-driver-sdk/go/driver"
	edit "github.com/bootun/legate-driver-sdk/go/protocol/openai/images/edits/v20260719"
	generation "github.com/bootun/legate-driver-sdk/go/protocol/openai/images/generations/v20260719"
)

type imageBinder struct{}

func (imageBinder) BindImage(request sdk.BindRequest) ([]byte, *sdk.DriverError) {
	if !sdk.IsEmptyJSONObject(request.ConfigJSON) || !contains(request.EndpointRefs, "primary") ||
		!configured(request.CredentialSlots, "api_key") {
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidConfig}
	}
	return []byte("openai-image-compatible-v1"), nil
}

type generationHandler struct{}

func (generationHandler) OpenAttempt(input generation.OpenAttemptInput) (*generation.OpenAttemptResult, *sdk.DriverError) {
	if string(input.BoundState) != "openai-image-compatible-v1" {
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidInvocation}
	}
	input.Request.Model = input.SelectedUpstreamModel
	body, err := input.Request.Encode()
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	return &generation.OpenAttemptResult{Attempt: &generationAttempt{}, Request: requestPlan("/images/generations", sdk.InlineBody(sdk.MediaTypeJSON, body))}, nil
}

type generationAttempt struct{ closed bool }

func (a *generationAttempt) TransformBuffered(upstream sdk.ImageUpstreamResponse) (*generation.ClientResponse, *sdk.DriverError) {
	if a.closed || upstream.Head == nil || upstream.Body == nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
	}
	var body generation.ResponseBody
	if upstream.Body.Blob != nil {
		body = generation.CompatibleBlobResponse{Blob: *upstream.Body.Blob}
	} else if upstream.Head.StatusCode >= 200 && upstream.Head.StatusCode < 400 {
		value, err := generation.DecodeSuccessResponse(upstream.Body.Inline)
		if err != nil {
			return nil, &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
		}
		body = value
	} else {
		value, err := generation.DecodeErrorResponse(upstream.Body.Inline)
		if err != nil {
			return nil, &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
		}
		body = value
	}
	response := generation.Response(upstream.Head.StatusCode, publicHeaders(upstream.Head.Headers), body).WithUsage(sdk.UnavailableUsage())
	return &response, nil
}

func (a *generationAttempt) Close() *sdk.DriverError { a.closed = true; return nil }

type editHandler struct{}

func (editHandler) OpenAttempt(input edit.OpenAttemptInput) (*edit.OpenAttemptResult, *sdk.DriverError) {
	if string(input.BoundState) != "openai-image-compatible-v1" {
		return nil, &sdk.DriverError{Code: sdk.ErrorInvalidInvocation}
	}
	body, err := input.Request.MultipartBody(input.SelectedUpstreamModel)
	if err != nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorDriverInternal}
	}
	return &edit.OpenAttemptResult{Attempt: &editAttempt{}, Request: requestPlan("/images/edits", body)}, nil
}

type editAttempt struct{ closed bool }

func (a *editAttempt) TransformBuffered(upstream sdk.ImageUpstreamResponse) (*edit.ClientResponse, *sdk.DriverError) {
	if a.closed || upstream.Head == nil || upstream.Body == nil {
		return nil, &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
	}
	var body edit.ResponseBody
	if upstream.Body.Blob != nil {
		body = edit.CompatibleBlobResponse{Blob: *upstream.Body.Blob}
	} else if upstream.Head.StatusCode >= 200 && upstream.Head.StatusCode < 400 {
		value, err := edit.DecodeSuccessResponse(upstream.Body.Inline)
		if err != nil {
			return nil, &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
		}
		body = value
	} else {
		value, err := edit.DecodeErrorResponse(upstream.Body.Inline)
		if err != nil {
			return nil, &sdk.DriverError{Code: sdk.ErrorMalformedUpstreamResponse}
		}
		body = value
	}
	response := edit.Response(upstream.Head.StatusCode, publicHeaders(upstream.Head.Headers), body).WithUsage(sdk.UnavailableUsage())
	return &response, nil
}

func (a *editAttempt) Close() *sdk.DriverError { a.closed = true; return nil }

func requestPlan(path string, body *sdk.BodyPlan) *sdk.RequestPlan {
	return &sdk.RequestPlan{EndpointRef: "primary", Method: "POST", RelativePath: path, BodyPlan: body,
		Auth: &sdk.AuthPlan{Kind: sdk.AuthKindBearer, CredentialSlot: "api_key"}}
}

func publicHeaders(input []sdk.NameValues) []sdk.NameValues {
	result := make([]sdk.NameValues, 0, len(input))
	for _, header := range input {
		if strings.EqualFold(header.Name, "content-type") || strings.EqualFold(header.Name, "content-length") {
			continue
		}
		result = append(result, header)
	}
	return result
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func configured(values []sdk.CredentialSlotDescriptor, wanted string) bool {
	for _, value := range values {
		if value.Name == wanted && value.Configured {
			return true
		}
	}
	return false
}
