package driver

import (
	"testing"
)

type testDriver struct {
	direct             bool
	omitDirectUsage    bool
	directAttemptClose *bool
}

func (testDriver) Bind(BindRequest) (*BindSuccess, *DriverError) {
	return &BindSuccess{TextCapabilities: &TextCapabilities{ProtocolContracts: []string{
		ProtocolContractOpenAIChatCompletions20260718,
	}}}, nil
}

func (d testDriver) OpenTextAttempt(request TextAttemptOpenRequest) (*TextAttemptOpenResult, *DriverError) {
	attempt := &testAttempt{contract: request.Invocation.Request.ProtocolContract, closed: d.directAttemptClose}
	if d.direct {
		response := Response(400, nil, ProtocolPayload{
			ProtocolContract: attempt.contract, MediaType: MediaTypeJSON, JSON: []byte(`{"error":{"message":"rejected","type":"invalid_request_error"}}`),
		}).WithOutcome(CallerError("unsupported_input"))
		if !d.omitDirectUsage {
			response.WithUsage(UnavailableUsage())
		}
		return &TextAttemptOpenResult{Attempt: attempt, Response: response}, nil
	}
	return &TextAttemptOpenResult{Attempt: attempt, Request: &RequestPlan{
		EndpointRef: "primary", Method: "POST", RelativePath: "/v1/chat/completions",
		Body: &MessageBody{MediaType: MediaTypeJSON, Payload: []byte(`{}`)}, Auth: &AuthPlan{Kind: AuthKindBearer, CredentialSlot: "api_key"},
	}}, nil
}

type testAttempt struct {
	contract string
	closed   *bool
}

func (a *testAttempt) TransformBufferedResponse(BufferedUpstreamResponse) (*TextTransformBufferedResponseSuccess, *DriverError) {
	response := Response(200, nil, ProtocolPayload{ProtocolContract: a.contract, MediaType: MediaTypeJSON, JSON: []byte(`{"id":"chatcmpl_1"}`)}).
		WithOutcome(Success()).WithUsage(UnavailableUsage())
	return &TextTransformBufferedResponseSuccess{Response: response}, nil
}
func (*testAttempt) OpenSSE(head UpstreamResponseHead) (*TextSSEOpenSuccess, *DriverError) {
	return &TextSSEOpenSuccess{StatusCode: head.StatusCode}, nil
}
func (a *testAttempt) TransformSSEEvent(UpstreamSSEEvent) (*TextSSETransformEventSuccess, *DriverError) {
	return &TextSSETransformEventSuccess{Events: []ProtocolEventPayload{{ProtocolContract: a.contract, EventType: "chunk", JSON: []byte(`{"id":"chatcmpl_1"}`)}}}, nil
}
func (*testAttempt) FinishSSE() (*TextSSEFinishSuccess, *DriverError) {
	usage := UnavailableUsage()
	outcome := Success()
	return &TextSSEFinishSuccess{Outcome: &outcome, Usage: &usage}, nil
}
func (a *testAttempt) Close() *DriverError {
	if a.closed != nil {
		*a.closed = true
	}
	return nil
}

func TestGuestOpenReturnsRequestPlanWithoutPrepare(t *testing.T) {
	guest := NewGuest(testDriver{})
	success := invokeOpen(t, guest, TextModeBuffered)
	if !hasField(success, 1) || !hasField(success, 2) || hasField(success, 3) {
		t.Fatalf("open success wire = %x", success)
	}
}

func TestGuestOpenMayReturnProtocolResponse(t *testing.T) {
	guest := NewGuest(testDriver{direct: true})
	success := invokeOpen(t, guest, TextModeBuffered)
	if !hasField(success, 1) || hasField(success, 2) || !hasField(success, 3) {
		t.Fatalf("open success wire = %x", success)
	}
}

func TestGuestRejectsImmediateResponseWithoutUsageAndClosesAttempt(t *testing.T) {
	closed := false
	guest := NewGuest(testDriver{direct: true, omitDirectUsage: true, directAttemptClose: &closed})
	wire := invokeOpenResponse(t, guest, TextModeBuffered)
	d := decoder{data: wire}
	field, _, err := d.key()
	if err != nil || field != 2 {
		t.Fatalf("open response = %x, err %v", wire, err)
	}
	if !closed {
		t.Fatal("invalid immediate response did not close its attempt")
	}
}

func TestCapabilitiesRequireSortedExactContracts(t *testing.T) {
	if validTextCapabilities(&TextCapabilities{ProtocolContracts: []string{
		ProtocolContractOpenAIResponses20260718,
		ProtocolContractOpenAIChatCompletions20260718,
	}}) {
		t.Fatal("unsorted contracts accepted")
	}
	if validTextCapabilities(&TextCapabilities{ProtocolContracts: []string{"openai.responses/latest"}}) {
		t.Fatal("unknown contract accepted")
	}
}

func TestOutcomeHasNoRetryDirective(t *testing.T) {
	for _, outcome := range []SemanticOutcome{Success(), CallerError("bad_input"), EndpointError("overloaded"), MappingError("model_not_found")} {
		if !validOutcome(&outcome) {
			t.Fatalf("outcome rejected: %#v", outcome)
		}
	}
}

func invokeOpen(t *testing.T, guest *Guest, mode TextMode) []byte {
	t.Helper()
	wire := invokeOpenResponse(t, guest, mode)
	d := decoder{data: wire}
	field, wireType, err := d.key()
	if err != nil || field != 1 {
		t.Fatalf("open response = %x, err %v", wire, err)
	}
	success, err := d.bytes(wireType)
	if err != nil {
		t.Fatal(err)
	}
	return success
}

func invokeOpenResponse(t *testing.T, guest *Guest, mode TextMode) []byte {
	t.Helper()
	requestPayload := encodeProtocolPayload(ProtocolPayload{ProtocolContract: ProtocolContractOpenAIChatCompletions20260718, MediaType: MediaTypeJSON, JSON: []byte(`{"model":"group"}`)})
	metadataPayload := encodeProtocolPayload(ProtocolPayload{ProtocolContract: ProtocolContractOpenAIChatCompletions20260718, MediaType: MediaTypeJSON, JSON: []byte(`{}`)})
	var invocation []byte
	invocation = appendEnum(invocation, 1, int32(mode))
	invocation = appendMessage(invocation, 2, requestPayload)
	invocation = appendMessage(invocation, 3, metadataPayload)
	invocation = appendString(invocation, 4, "gpt-upstream")
	invocation = appendString(invocation, 5, "resp_1")
	return guest.HandleTextAttemptOpen(appendMessage(nil, 2, invocation))
}

func hasField(data []byte, want int) bool {
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return false
		}
		if field == want {
			return true
		}
		if err := d.skip(field, wire); err != nil {
			return false
		}
	}
	return false
}
