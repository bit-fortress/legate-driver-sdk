package v20260718

import (
	"errors"

	driver "github.com/bootun/legate-driver-sdk/go/driver"
)

type OpenAttemptInput struct {
	BoundState            []byte
	Mode                  driver.TextMode
	Request               Request
	ProtocolMetadata      ProtocolMetadata
	SelectedUpstreamModel string
	ResponseID            string
}
type ResponseBody interface{ protocolResponseBody() ([]byte, error) }

func (value BufferedSuccessResponse) protocolResponseBody() ([]byte, error) {
	decoded, err := DecodeBufferedSuccess(value.JSON)
	return append([]byte(nil), decoded.JSON...), err
}
func (value ErrorResponse) protocolResponseBody() ([]byte, error) {
	decoded, err := DecodeError(value.JSON)
	return append([]byte(nil), decoded.JSON...), err
}

type ClientResponse struct {
	StatusCode int32
	Headers    []driver.NameValues
	Body       ResponseBody
	Outcome    *driver.SemanticOutcome
	Usage      *driver.UsageReport
}

// Response builds a typed response. Attach Usage before returning it.
func Response(statusCode int32, headers []driver.NameValues, body ResponseBody) ClientResponse {
	return ClientResponse{StatusCode: statusCode, Headers: headers, Body: body}
}
func (response ClientResponse) WithOutcome(outcome driver.SemanticOutcome) ClientResponse {
	response.Outcome = &outcome
	return response
}
func (response ClientResponse) WithUsage(usage driver.UsageReport) ClientResponse {
	response.Usage = &usage
	return response
}

type OpenAttemptResult struct {
	Attempt  Attempt
	Request  *driver.RequestPlan
	Response *ClientResponse
}
type StreamEventResult struct {
	Events  []StreamEvent
	Outcome *driver.SemanticOutcome
	Usage   *driver.UsageReport
}
type StreamFinishResult = StreamEventResult
type Handler interface {
	OpenAttempt(OpenAttemptInput) (*OpenAttemptResult, *driver.DriverError)
}
type Attempt interface {
	TransformBuffered(driver.BufferedUpstreamResponse) (*ClientResponse, *driver.DriverError)
	OpenStream(driver.UpstreamResponseHead) (*driver.TextSSEOpenSuccess, *driver.DriverError)
	TransformStreamEvent(driver.UpstreamSSEEvent) (*StreamEventResult, *driver.DriverError)
	FinishStream() (*StreamFinishResult, *driver.DriverError)
	Close() *driver.DriverError
}

func Register(handler Handler) driver.ProtocolHandlerRegistration {
	return driver.NewProtocolHandlerRegistration(Contract, func(request driver.TextAttemptOpenRequest) (*driver.TextAttemptOpenResult, *driver.DriverError) {
		if handler == nil || request.Invocation == nil || request.Invocation.Request == nil || request.Invocation.Request.ProtocolContract != Contract {
			return nil, &driver.DriverError{Code: driver.ErrorInvalidInvocation}
		}
		invocation := request.Invocation
		typedRequest, err := DecodeRequest(invocation.Request.JSON)
		if err != nil {
			return nil, &driver.DriverError{Code: driver.ErrorInvalidProtocolRequest}
		}
		metadataJSON := []byte("{}")
		if invocation.ProtocolMetadata != nil {
			if invocation.ProtocolMetadata.ProtocolContract != Contract {
				return nil, &driver.DriverError{Code: driver.ErrorInvalidInvocation}
			}
			metadataJSON = invocation.ProtocolMetadata.JSON
		}
		metadata, err := DecodeMetadata(metadataJSON)
		if err != nil {
			return nil, &driver.DriverError{Code: driver.ErrorInvalidProtocolRequest}
		}
		result, failure := handler.OpenAttempt(OpenAttemptInput{BoundState: append([]byte(nil), request.BoundState...), Mode: invocation.Mode,
			Request: typedRequest, ProtocolMetadata: metadata, SelectedUpstreamModel: invocation.SelectedUpstreamModel, ResponseID: invocation.ResponseID})
		if failure != nil {
			return nil, failure
		}
		if result == nil || result.Attempt == nil || (result.Request == nil) == (result.Response == nil) {
			return nil, &driver.DriverError{Code: driver.ErrorDriverInternal}
		}
		raw := &driver.TextAttemptOpenResult{Attempt: attemptAdapter{attempt: result.Attempt}, Request: result.Request}
		if result.Response != nil {
			converted, err := rawResponse(*result.Response)
			if err != nil {
				_ = result.Attempt.Close()
				return nil, &driver.DriverError{Code: driver.ErrorDriverInternal}
			}
			raw.Response = converted
		}
		return raw, nil
	})
}

type attemptAdapter struct{ attempt Attempt }

func (adapter attemptAdapter) TransformBufferedResponse(upstream driver.BufferedUpstreamResponse) (*driver.TextTransformBufferedResponseSuccess, *driver.DriverError) {
	response, failure := adapter.attempt.TransformBuffered(upstream)
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
	return &driver.TextTransformBufferedResponseSuccess{Response: converted}, nil
}
func (adapter attemptAdapter) OpenSSE(upstream driver.UpstreamResponseHead) (*driver.TextSSEOpenSuccess, *driver.DriverError) {
	return adapter.attempt.OpenStream(upstream)
}
func (adapter attemptAdapter) TransformSSEEvent(upstream driver.UpstreamSSEEvent) (*driver.TextSSETransformEventSuccess, *driver.DriverError) {
	result, failure := adapter.attempt.TransformStreamEvent(upstream)
	if failure != nil {
		return nil, failure
	}
	return rawStreamResult(result)
}
func (adapter attemptAdapter) FinishSSE() (*driver.TextSSEFinishSuccess, *driver.DriverError) {
	result, failure := adapter.attempt.FinishStream()
	if failure != nil {
		return nil, failure
	}
	converted, failure := rawStreamResult(result)
	if failure != nil {
		return nil, failure
	}
	return &driver.TextSSEFinishSuccess{Events: converted.Events, Outcome: converted.Outcome, Usage: converted.Usage}, nil
}
func (adapter attemptAdapter) Close() *driver.DriverError { return adapter.attempt.Close() }
func rawResponse(response ClientResponse) (*driver.ClientResponse, error) {
	if response.Body == nil {
		return nil, errors.New("response body is required")
	}
	body, err := response.Body.protocolResponseBody()
	if err != nil {
		return nil, err
	}
	return &driver.ClientResponse{StatusCode: response.StatusCode, Headers: response.Headers,
		Body: &driver.ProtocolPayload{ProtocolContract: Contract, MediaType: driver.MediaTypeJSON, JSON: body}, Outcome: response.Outcome, Usage: response.Usage}, nil
}
func rawStreamResult(result *StreamEventResult) (*driver.TextSSETransformEventSuccess, *driver.DriverError) {
	if result == nil {
		return nil, &driver.DriverError{Code: driver.ErrorDriverInternal}
	}
	events := make([]driver.ProtocolEventPayload, 0, len(result.Events))
	for _, event := range result.Events {
		decoded, err := DecodeStreamEvent(event.EventType, event.JSON)
		if err != nil {
			return nil, &driver.DriverError{Code: driver.ErrorInvalidProtocolResponse}
		}
		events = append(events, driver.ProtocolEventPayload{ProtocolContract: Contract, EventType: decoded.EventType, JSON: append([]byte(nil), decoded.JSON...)})
	}
	return &driver.TextSSETransformEventSuccess{Events: events, Outcome: result.Outcome, Usage: result.Usage}, nil
}
