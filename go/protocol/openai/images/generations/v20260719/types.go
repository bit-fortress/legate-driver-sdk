package v20260719

import (
	"encoding/json"
	"errors"

	"github.com/bootun/legate-driver-sdk/go/protocol/internal/strictjson"
)

const Contract = "openai.images.generations/2026-07-19"

type Request struct {
	Model             string
	Prompt            string
	N                 *int
	Quality           *string
	ResponseFormat    *string
	Size              *string
	Style             *string
	User              *string
	OutputFormat      *string
	OutputCompression *int
	Background        *string
	Moderation        *string
	ExtraFields       map[string]json.RawMessage
}

type ProtocolMetadata struct{ ExtraFields map[string]json.RawMessage }

type ImageData struct {
	B64JSON       *string
	URL           *string
	RevisedPrompt *string
	ExtraFields   map[string]json.RawMessage
}

type SuccessResponse struct {
	Created     *int64
	Data        []ImageData
	Usage       json.RawMessage
	ExtraFields map[string]json.RawMessage
}

type ErrorDetail struct {
	Message     string
	Type        string
	Param       json.RawMessage
	Code        json.RawMessage
	ExtraFields map[string]json.RawMessage
}

type ErrorResponse struct {
	Error       ErrorDetail
	ExtraFields map[string]json.RawMessage
}

func DecodeRequest(input []byte) (Request, error) {
	fields, err := strictjson.DecodeObject(input)
	if err != nil {
		return Request{}, err
	}
	result := Request{ExtraFields: fields}
	if err := takeRequiredString(fields, "model", &result.Model); err != nil {
		return Request{}, err
	}
	if err := takeRequiredString(fields, "prompt", &result.Prompt); err != nil {
		return Request{}, err
	}
	for name, target := range map[string]**string{
		"quality": &result.Quality, "response_format": &result.ResponseFormat, "size": &result.Size,
		"style": &result.Style, "user": &result.User, "output_format": &result.OutputFormat,
		"background": &result.Background, "moderation": &result.Moderation,
	} {
		if err := takeOptional(fields, name, target); err != nil {
			return Request{}, err
		}
	}
	if err := takeOptional(fields, "n", &result.N); err != nil {
		return Request{}, err
	}
	if err := takeOptional(fields, "output_compression", &result.OutputCompression); err != nil {
		return Request{}, err
	}
	return result, nil
}

func (r Request) Encode() ([]byte, error) {
	if r.Model == "" || r.Prompt == "" {
		return nil, errors.New("model and prompt are required")
	}
	fields := cloneFields(r.ExtraFields)
	fields["model"] = mustJSON(r.Model)
	fields["prompt"] = mustJSON(r.Prompt)
	putOptional(fields, "n", r.N)
	putOptional(fields, "quality", r.Quality)
	putOptional(fields, "response_format", r.ResponseFormat)
	putOptional(fields, "size", r.Size)
	putOptional(fields, "style", r.Style)
	putOptional(fields, "user", r.User)
	putOptional(fields, "output_format", r.OutputFormat)
	putOptional(fields, "output_compression", r.OutputCompression)
	putOptional(fields, "background", r.Background)
	putOptional(fields, "moderation", r.Moderation)
	return json.Marshal(fields)
}

func DecodeMetadata(input []byte) (ProtocolMetadata, error) {
	fields, err := strictjson.DecodeObject(input)
	return ProtocolMetadata{ExtraFields: fields}, err
}

func DecodeSuccessResponse(input []byte) (SuccessResponse, error) {
	fields, err := strictjson.DecodeObject(input)
	if err != nil {
		return SuccessResponse{}, err
	}
	result := SuccessResponse{ExtraFields: fields}
	if err := takeOptional(fields, "created", &result.Created); err != nil {
		return SuccessResponse{}, err
	}
	if raw, ok := fields["data"]; ok {
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return SuccessResponse{}, errors.New("data must be an array")
		}
		delete(fields, "data")
		result.Data = make([]ImageData, len(items))
		for index, item := range items {
			result.Data[index].ExtraFields = item
			if err := takeOptional(item, "b64_json", &result.Data[index].B64JSON); err != nil {
				return SuccessResponse{}, err
			}
			if err := takeOptional(item, "url", &result.Data[index].URL); err != nil {
				return SuccessResponse{}, err
			}
			if err := takeOptional(item, "revised_prompt", &result.Data[index].RevisedPrompt); err != nil {
				return SuccessResponse{}, err
			}
		}
	}
	if raw, ok := fields["usage"]; ok {
		result.Usage = append([]byte(nil), raw...)
		delete(fields, "usage")
	}
	return result, nil
}

func (r SuccessResponse) Encode() ([]byte, error) {
	fields := cloneFields(r.ExtraFields)
	putOptional(fields, "created", r.Created)
	if r.Data != nil {
		items := make([]map[string]json.RawMessage, len(r.Data))
		for index, item := range r.Data {
			fields := cloneFields(item.ExtraFields)
			putOptional(fields, "b64_json", item.B64JSON)
			putOptional(fields, "url", item.URL)
			putOptional(fields, "revised_prompt", item.RevisedPrompt)
			items[index] = fields
		}
		fields["data"] = mustJSON(items)
	}
	if r.Usage != nil {
		fields["usage"] = append([]byte(nil), r.Usage...)
	}
	return json.Marshal(fields)
}

func DecodeErrorResponse(input []byte) (ErrorResponse, error) {
	fields, err := strictjson.DecodeObject(input)
	if err != nil {
		return ErrorResponse{}, err
	}
	raw, ok := fields["error"]
	if !ok {
		return ErrorResponse{}, errors.New("error is required")
	}
	inner, err := strictjson.DecodeObject(raw)
	if err != nil {
		return ErrorResponse{}, err
	}
	result := ErrorResponse{ExtraFields: fields, Error: ErrorDetail{ExtraFields: inner}}
	delete(fields, "error")
	if err := takeRequiredString(inner, "message", &result.Error.Message); err != nil {
		return ErrorResponse{}, err
	}
	_ = takeOptionalValue(inner, "type", &result.Error.Type)
	if value, ok := inner["param"]; ok {
		result.Error.Param = append([]byte(nil), value...)
		delete(inner, "param")
	}
	if value, ok := inner["code"]; ok {
		result.Error.Code = append([]byte(nil), value...)
		delete(inner, "code")
	}
	return result, nil
}

func (r ErrorResponse) Encode() ([]byte, error) {
	if r.Error.Message == "" {
		return nil, errors.New("error message is required")
	}
	inner := cloneFields(r.Error.ExtraFields)
	inner["message"] = mustJSON(r.Error.Message)
	if r.Error.Type != "" {
		inner["type"] = mustJSON(r.Error.Type)
	}
	if r.Error.Param != nil {
		inner["param"] = append([]byte(nil), r.Error.Param...)
	}
	if r.Error.Code != nil {
		inner["code"] = append([]byte(nil), r.Error.Code...)
	}
	fields := cloneFields(r.ExtraFields)
	fields["error"] = mustJSON(inner)
	return json.Marshal(fields)
}

func takeRequiredString(fields map[string]json.RawMessage, name string, target *string) error {
	raw, ok := fields[name]
	if !ok || json.Unmarshal(raw, target) != nil || *target == "" {
		return errors.New(name + " must be a non-empty string")
	}
	delete(fields, name)
	return nil
}

func takeOptional[T any](fields map[string]json.RawMessage, name string, target **T) error {
	raw, ok := fields[name]
	if !ok {
		return nil
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return errors.New(name + " has an invalid type")
	}
	*target = &value
	delete(fields, name)
	return nil
}

func takeOptionalValue(fields map[string]json.RawMessage, name string, target *string) error {
	raw, ok := fields[name]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	delete(fields, name)
	return nil
}

func putOptional[T any](fields map[string]json.RawMessage, name string, value *T) {
	if value != nil {
		fields[name] = mustJSON(*value)
	}
}

func cloneFields(input map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(input)+4)
	for key, value := range input {
		result[key] = append([]byte(nil), value...)
	}
	return result
}

func mustJSON(value any) json.RawMessage { encoded, _ := json.Marshal(value); return encoded }
