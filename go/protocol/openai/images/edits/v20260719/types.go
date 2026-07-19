package v20260719

import (
	"errors"
	"strings"
	"unicode/utf8"

	driver "github.com/bootun/legate-driver-sdk/go/driver"
	gen "github.com/bootun/legate-driver-sdk/go/protocol/openai/images/generations/v20260719"
)

const Contract = "openai.images.edits/2026-07-19"

type Request struct {
	Parts      []driver.MultipartInputPart
	Model      string
	Prompt     string
	ImageParts []int
	MaskPart   *int
}

type ProtocolMetadata struct{}

type SuccessResponse gen.SuccessResponse
type ErrorResponse gen.ErrorResponse

func DecodeSuccessResponse(input []byte) (SuccessResponse, error) {
	value, err := gen.DecodeSuccessResponse(input)
	return SuccessResponse(value), err
}

func (r SuccessResponse) Encode() ([]byte, error) { return gen.SuccessResponse(r).Encode() }

func DecodeErrorResponse(input []byte) (ErrorResponse, error) {
	value, err := gen.DecodeErrorResponse(input)
	return ErrorResponse(value), err
}

func (r ErrorResponse) Encode() ([]byte, error) { return gen.ErrorResponse(r).Encode() }

func DecodeRequest(input *driver.MultipartInput) (Request, error) {
	if input == nil || len(input.Parts) == 0 {
		return Request{}, errors.New("multipart request is required")
	}
	result := Request{Parts: cloneParts(input.Parts)}
	for index, part := range result.Parts {
		switch part.Name {
		case "model", "prompt":
			if part.Filename != nil || part.Blob != nil || !utf8.Valid(part.Inline) {
				return Request{}, errors.New(part.Name + " must be a text field")
			}
			if part.Name == "model" && result.Model == "" {
				result.Model = string(part.Inline)
			}
			if part.Name == "prompt" && result.Prompt == "" {
				result.Prompt = string(part.Inline)
			}
		case "image":
			if part.Filename == nil || part.Blob == nil || part.Inline != nil {
				return Request{}, errors.New("image must use a file part")
			}
			result.ImageParts = append(result.ImageParts, index)
		case "mask":
			if part.Filename == nil || part.Blob == nil || part.Inline != nil || result.MaskPart != nil {
				return Request{}, errors.New("mask must use one file part")
			}
			position := index
			result.MaskPart = &position
		}
	}
	if strings.TrimSpace(result.Model) == "" || strings.TrimSpace(result.Prompt) == "" || len(result.ImageParts) == 0 {
		return Request{}, errors.New("model, prompt, and image are required")
	}
	return result, nil
}

func (r Request) MultipartBody(selectedUpstreamModel string) (*driver.BodyPlan, error) {
	if selectedUpstreamModel == "" || len(r.Parts) == 0 {
		return nil, errors.New("selected upstream model and multipart parts are required")
	}
	body := driver.NewMultipartBody("multipart/form-data")
	sawModel := false
	for _, input := range r.Parts {
		part := driver.MultipartBodyPart{
			Name: input.Name, Filename: cloneString(input.Filename), Headers: filterPartHeaders(input.Headers),
			ContentType: input.ContentType,
		}
		if input.Name == "model" && input.Filename == nil {
			sawModel = true
			part.Content = driver.InlineSegment([]byte(selectedUpstreamModel))
		} else if input.Blob != nil {
			part.Content = driver.BlobSegment(*input.Blob)
		} else {
			part.Content = driver.InlineSegment(input.Inline)
		}
		body.AddPart(part)
	}
	if !sawModel {
		body.AddPart(driver.MultipartBodyPart{Name: "model", Content: driver.InlineSegment([]byte(selectedUpstreamModel))})
	}
	return body, nil
}

func cloneParts(input []driver.MultipartInputPart) []driver.MultipartInputPart {
	result := make([]driver.MultipartInputPart, len(input))
	for index, part := range input {
		result[index] = part
		result[index].Filename = cloneString(part.Filename)
		result[index].Headers = append([]driver.NameValues(nil), part.Headers...)
		result[index].Inline = append([]byte(nil), part.Inline...)
		if part.Blob != nil {
			ref := *part.Blob
			result[index].Blob = &ref
		}
	}
	return result
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func filterPartHeaders(input []driver.NameValues) []driver.NameValues {
	result := make([]driver.NameValues, 0, len(input))
	for _, header := range input {
		if strings.EqualFold(header.Name, "content-disposition") || strings.EqualFold(header.Name, "content-type") ||
			strings.EqualFold(header.Name, "content-length") {
			continue
		}
		result = append(result, header)
	}
	return result
}
