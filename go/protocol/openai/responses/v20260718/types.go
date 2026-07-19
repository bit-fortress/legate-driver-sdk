// Package v20260718 contains the OpenAI Responses protocol contract.
package v20260718

import (
	"encoding/json"
	"errors"

	"github.com/bootun/legate-driver-sdk/go/protocol/internal/strictjson"
)

const Contract = "openai.responses/2026-07-18"

type Request struct {
	Model       string
	Input       json.RawMessage
	Stream      bool
	ExtraFields map[string]json.RawMessage
}
type ProtocolMetadata struct{ ExtraFields map[string]json.RawMessage }
type BufferedSuccessResponse struct{ JSON json.RawMessage }
type ErrorResponse struct{ JSON json.RawMessage }
type StreamEvent struct {
	EventType string
	JSON      json.RawMessage
}
type StreamTermination struct{ EventType string }

func DecodeRequest(input []byte) (Request, error) {
	fields, err := decodeObject(input)
	if err != nil {
		return Request{}, err
	}
	result := Request{ExtraFields: fields}
	if raw, ok := fields["model"]; ok {
		if json.Unmarshal(raw, &result.Model) != nil {
			return Request{}, errors.New("model must be a string")
		}
		delete(fields, "model")
	}
	if result.Model == "" {
		return Request{}, errors.New("model is required")
	}
	if raw, ok := fields["input"]; ok {
		result.Input = append([]byte(nil), raw...)
		delete(fields, "input")
	}
	if len(result.Input) == 0 {
		return Request{}, errors.New("input is required")
	}
	if raw, ok := fields["stream"]; ok {
		if json.Unmarshal(raw, &result.Stream) != nil {
			return Request{}, errors.New("stream must be a boolean")
		}
		delete(fields, "stream")
	}
	return result, nil
}
func (r Request) Encode() ([]byte, error) {
	fields := cloneFields(r.ExtraFields)
	fields["model"] = mustJSON(r.Model)
	fields["input"] = append([]byte(nil), r.Input...)
	fields["stream"] = mustJSON(r.Stream)
	return json.Marshal(fields)
}
func DecodeMetadata(input []byte) (ProtocolMetadata, error) {
	fields, err := decodeObject(input)
	return ProtocolMetadata{ExtraFields: fields}, err
}
func DecodeBufferedSuccess(input []byte) (BufferedSuccessResponse, error) {
	if _, err := decodeObject(input); err != nil {
		return BufferedSuccessResponse{}, err
	}
	return BufferedSuccessResponse{JSON: append([]byte(nil), input...)}, nil
}
func DecodeError(input []byte) (ErrorResponse, error) {
	if _, err := decodeObject(input); err != nil {
		return ErrorResponse{}, err
	}
	return ErrorResponse{JSON: append([]byte(nil), input...)}, nil
}
func DecodeStreamEvent(eventType string, input []byte) (StreamEvent, error) {
	if eventType == "" {
		return StreamEvent{}, errors.New("event type is required")
	}
	if _, err := decodeObject(input); err != nil {
		return StreamEvent{}, err
	}
	return StreamEvent{EventType: eventType, JSON: append([]byte(nil), input...)}, nil
}
func decodeObject(input []byte) (map[string]json.RawMessage, error) {
	return strictjson.DecodeObject(input)
}
func cloneFields(fields map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(fields)+3)
	for key, value := range fields {
		result[key] = append([]byte(nil), value...)
	}
	return result
}
func mustJSON(value any) json.RawMessage { encoded, _ := json.Marshal(value); return encoded }
