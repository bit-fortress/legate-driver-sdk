package v20260719

import (
	"encoding/json"
	"errors"

	driver "github.com/bootun/legate-driver-sdk/go/driver"
	gen "github.com/bootun/legate-driver-sdk/go/protocol/openai/images/generations/v20260719"
)

type OpenAttemptInput struct {
	BoundState            []byte
	Request               Request
	ProtocolMetadata      ProtocolMetadata
	SelectedUpstreamModel string
	ResponseID            string
	Blobs                 []driver.BlobRef
}

type ResponseBody interface {
	bodyPlan() (*driver.BodyPlan, error)
}

func (value SuccessResponse) bodyPlan() (*driver.BodyPlan, error) {
	encoded, err := value.Encode()
	return driver.InlineBody(driver.MediaTypeJSON, encoded), err
}

func (value ErrorResponse) bodyPlan() (*driver.BodyPlan, error) {
	encoded, err := value.Encode()
	return driver.InlineBody(driver.MediaTypeJSON, encoded), err
}

type CompatibleBlobResponse struct{ Blob driver.BlobRef }

func (value CompatibleBlobResponse) bodyPlan() (*driver.BodyPlan, error) {
	return driver.BlobBody(driver.MediaTypeJSON, value.Blob), nil
}

type ClientResponse struct {
	StatusCode int32
	Headers    []driver.NameValues
	Body       ResponseBody
	Outcome    *driver.SemanticOutcome
	Usage      *driver.UsageReport
}

func Response(statusCode int32, headers []driver.NameValues, body ResponseBody) ClientResponse {
	return ClientResponse{StatusCode: statusCode, Headers: headers, Body: body}
}
func (r ClientResponse) WithOutcome(outcome driver.SemanticOutcome) ClientResponse {
	r.Outcome = &outcome
	return r
}
func (r ClientResponse) WithUsage(usage driver.UsageReport) ClientResponse {
	r.Usage = &usage
	return r
}

type OpenAttemptResult struct {
	Attempt  Attempt
	Request  *driver.RequestPlan
	Response *ClientResponse
}

type Handler interface {
	OpenAttempt(OpenAttemptInput) (*OpenAttemptResult, *driver.DriverError)
}
type Attempt interface {
	TransformBuffered(driver.ImageUpstreamResponse) (*ClientResponse, *driver.DriverError)
	Close() *driver.DriverError
}

func Register(handler Handler) driver.ImageProtocolHandlerRegistration {
	return driver.NewImageProtocolHandlerRegistration(Contract, func(request driver.ImageAttemptOpenRequest) (*driver.ImageAttemptOpenResult, *driver.DriverError) {
		if handler == nil || request.Invocation == nil || request.Invocation.Request == nil ||
			request.Invocation.Request.ProtocolContract != Contract {
			return nil, &driver.DriverError{Code: driver.ErrorInvalidInvocation}
		}
		invocation := request.Invocation
		typedRequest, err := DecodeRequest(invocation.Request.Multipart)
		if err != nil {
			return invalidOpenResponse(), nil
		}
		result, failure := handler.OpenAttempt(OpenAttemptInput{
			BoundState: append([]byte(nil), request.BoundState...), Request: typedRequest,
			SelectedUpstreamModel: invocation.SelectedUpstreamModel, ResponseID: invocation.ResponseID,
			Blobs: append([]driver.BlobRef(nil), invocation.Blobs...),
		})
		if failure != nil {
			return nil, failure
		}
		if result == nil || result.Attempt == nil || (result.Request == nil) == (result.Response == nil) {
			return nil, &driver.DriverError{Code: driver.ErrorDriverInternal}
		}
		raw := &driver.ImageAttemptOpenResult{Attempt: attemptAdapter{attempt: result.Attempt}, Request: result.Request}
		if result.Response != nil {
			raw.Response, err = rawResponse(*result.Response)
			if err != nil {
				_ = result.Attempt.Close()
				return nil, &driver.DriverError{Code: driver.ErrorDriverInternal}
			}
		}
		return raw, nil
	})
}

type attemptAdapter struct{ attempt Attempt }

func (a attemptAdapter) TransformBufferedResponse(upstream driver.ImageUpstreamResponse) (*driver.ImageTransformBufferedResponseSuccess, *driver.DriverError) {
	response, failure := a.attempt.TransformBuffered(upstream)
	if failure != nil {
		return nil, failure
	}
	if response == nil {
		return nil, &driver.DriverError{Code: driver.ErrorDriverInternal}
	}
	converted, err := rawResponse(*response)
	if err != nil {
		return nil, &driver.DriverError{Code: driver.ErrorInvalidProtocolResponse}
	}
	return &driver.ImageTransformBufferedResponseSuccess{Response: converted}, nil
}
func (a attemptAdapter) Close() *driver.DriverError { return a.attempt.Close() }

type rejectedAttempt struct{}

func (rejectedAttempt) TransformBuffered(driver.ImageUpstreamResponse) (*ClientResponse, *driver.DriverError) {
	return nil, &driver.DriverError{Code: driver.ErrorInvalidAttemptState}
}
func (rejectedAttempt) Close() *driver.DriverError { return nil }

type rawRejectedAttempt struct{}

func (rawRejectedAttempt) TransformBufferedResponse(driver.ImageUpstreamResponse) (*driver.ImageTransformBufferedResponseSuccess, *driver.DriverError) {
	return nil, &driver.DriverError{Code: driver.ErrorInvalidAttemptState}
}
func (rawRejectedAttempt) Close() *driver.DriverError { return nil }

func invalidOpenResponse() *driver.ImageAttemptOpenResult {
	body := ErrorResponse(gen.ErrorResponse{Error: gen.ErrorDetail{
		Message: "Invalid image request.", Type: "invalid_request_error", Code: json.RawMessage(`"invalid_request"`),
	}})
	response := Response(400, nil, body).WithOutcome(driver.CallerError("invalid_request")).WithUsage(driver.UnavailableUsage())
	raw, _ := rawResponse(response)
	return &driver.ImageAttemptOpenResult{Attempt: rawRejectedAttempt{}, Response: raw}
}

func rawResponse(response ClientResponse) (*driver.ImageClientResponse, error) {
	if response.Body == nil || response.Usage == nil {
		return nil, errors.New("response body and usage are required")
	}
	body, err := response.Body.bodyPlan()
	if err != nil {
		return nil, err
	}
	return &driver.ImageClientResponse{StatusCode: response.StatusCode, Headers: response.Headers,
		ProtocolContract: Contract, Body: body, Outcome: response.Outcome, Usage: response.Usage}, nil
}
