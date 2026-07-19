package v20260718

import (
	"testing"

	driver "github.com/bootun/legate-driver-sdk/go/driver"
)

type dispatchTestHandler struct{ model string }

func (handler *dispatchTestHandler) OpenAttempt(input OpenAttemptInput) (*OpenAttemptResult, *driver.DriverError) {
	handler.model = input.Request.Model
	body, err := DecodeError([]byte(`{"error":{"message":"stopped"}}`))
	if err != nil {
		panic(err)
	}
	response := Response(400, nil, body).WithUsage(driver.UnavailableUsage())
	return &OpenAttemptResult{Attempt: dispatchTestAttempt{}, Response: &response}, nil
}

type dispatchTestAttempt struct{}

func (dispatchTestAttempt) TransformBuffered(driver.BufferedUpstreamResponse) (*ClientResponse, *driver.DriverError) {
	return nil, &driver.DriverError{Code: driver.ErrorMalformedUpstreamResponse}
}
func (dispatchTestAttempt) OpenStream(driver.UpstreamResponseHead) (*driver.TextSSEOpenSuccess, *driver.DriverError) {
	return &driver.TextSSEOpenSuccess{StatusCode: 200}, nil
}
func (dispatchTestAttempt) TransformStreamEvent(driver.UpstreamSSEEvent) (*StreamEventResult, *driver.DriverError) {
	return &StreamEventResult{}, nil
}
func (dispatchTestAttempt) FinishStream() (*StreamFinishResult, *driver.DriverError) {
	return &StreamFinishResult{}, nil
}
func (dispatchTestAttempt) Close() *driver.DriverError { return nil }

func TestTypedHandlerDispatchesExactContract(t *testing.T) {
	handler := &dispatchTestHandler{}
	dispatcher, err := driver.NewDispatcher([]string{Contract},
		driver.TextBinderFunc(func(driver.BindRequest) ([]byte, *driver.DriverError) { return nil, nil }),
		Register(handler),
	)
	if err != nil {
		t.Fatal(err)
	}
	result, failure := dispatcher.OpenTextAttempt(driver.TextAttemptOpenRequest{Invocation: &driver.TextInvocation{
		Mode: driver.TextModeBuffered, SelectedUpstreamModel: "upstream", ResponseID: "resp_1",
		Request: &driver.ProtocolPayload{ProtocolContract: Contract, MediaType: driver.MediaTypeJSON,
			JSON: []byte(`{"model":"public","messages":[]}`)},
	}})
	if failure != nil || result == nil || result.Response == nil || handler.model != "public" {
		t.Fatalf("result = %#v failure = %#v model = %q", result, failure, handler.model)
	}
}

func TestTypedHandlerRejectsDuplicateKeysBeforeDispatch(t *testing.T) {
	handler := &dispatchTestHandler{}
	dispatcher := driver.MustNewDispatcher([]string{Contract},
		driver.TextBinderFunc(func(driver.BindRequest) ([]byte, *driver.DriverError) { return nil, nil }),
		Register(handler),
	)
	_, failure := dispatcher.OpenTextAttempt(driver.TextAttemptOpenRequest{Invocation: &driver.TextInvocation{
		Mode: driver.TextModeBuffered, SelectedUpstreamModel: "upstream", ResponseID: "resp_1",
		Request: &driver.ProtocolPayload{ProtocolContract: Contract, MediaType: driver.MediaTypeJSON,
			JSON: []byte(`{"model":"one","model":"two","messages":[]}`)},
	}})
	if failure == nil || failure.Code != driver.ErrorInvalidProtocolRequest || handler.model != "" {
		t.Fatalf("failure = %#v model = %q", failure, handler.model)
	}
}
