package driver

import (
	"math"
	"sort"
	"unicode"
	"unicode/utf8"
)

// Driver binds endpoint configuration and creates stateful text attempts. It
// never receives credential values and must not perform network I/O.
type Driver interface {
	Bind(BindRequest) (*BindSuccess, *DriverError)
	OpenTextAttempt(TextAttemptOpenRequest) (*TextAttemptOpenResult, *DriverError)
}

type TextAttemptOpenResult struct {
	Attempt  TextAttempt
	Request  *RequestPlan
	Response *ClientResponse
}

// TextAttempt belongs to one real upstream candidate attempt. The Host invokes
// methods sequentially and calls Close exactly once.
type TextAttempt interface {
	TransformBufferedResponse(BufferedUpstreamResponse) (*TextTransformBufferedResponseSuccess, *DriverError)
	OpenSSE(UpstreamResponseHead) (*TextSSEOpenSuccess, *DriverError)
	TransformSSEEvent(UpstreamSSEEvent) (*TextSSETransformEventSuccess, *DriverError)
	FinishSSE() (*TextSSEFinishSuccess, *DriverError)
	Close() *DriverError
}

// Guest owns the ABI handle for the single active attempt in one module
// instance. Guest is not safe for concurrent use; Core serializes hook calls.
type Guest struct {
	driver     Driver
	attempt    TextAttempt
	handle     uint64
	nextHandle uint64
	mode       TextMode
	phase      attemptPhase
	sawOutcome bool
	sawUsage   bool
}

type attemptPhase uint8

const (
	attemptPhaseNone attemptPhase = iota
	attemptPhaseReady
	attemptPhaseSSEOpen
	attemptPhaseTerminal
)

func NewGuest(driver Driver) *Guest { return &Guest{driver: driver, nextHandle: 1} }

// IsEmptyJSONObject reports whether input is a JSON object containing only whitespace.
func IsEmptyJSONObject(input []byte) bool {
	start := 0
	for start < len(input) && isJSONWhitespace(input[start]) {
		start++
	}
	end := len(input)
	for end > start && isJSONWhitespace(input[end-1]) {
		end--
	}
	if end-start < 2 || input[start] != '{' || input[end-1] != '}' {
		return false
	}
	for _, current := range input[start+1 : end-1] {
		if !isJSONWhitespace(current) {
			return false
		}
	}
	return true
}

func isJSONWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r'
}

func (g *Guest) HandleBind(input []byte) []byte {
	request, err := decodeBindRequest(input)
	if err != nil || g == nil || g.driver == nil {
		return encodeBindResponse(nil, internalError())
	}
	success, driverError := g.driver.Bind(request)
	if invalidResult(success, driverError) || success != nil &&
		(!validTextCapabilities(success.TextCapabilities) || success.ImageCapabilities != nil) {
		return encodeBindResponse(nil, internalError())
	}
	return encodeBindResponse(success, driverError)
}

func (g *Guest) HandleTextAttemptOpen(input []byte) []byte {
	request, err := decodeTextAttemptOpenRequest(input)
	if err != nil || g == nil || g.driver == nil {
		return encodeTextAttemptOpenResponse(nil, internalError())
	}
	if !validInvocation(request.Invocation) {
		return encodeTextAttemptOpenResponse(nil, &DriverError{Code: ErrorInvalidInvocation})
	}
	if request.Invocation.Mode != TextModeBuffered && request.Invocation.Mode != TextModeSSE {
		return encodeTextAttemptOpenResponse(nil, &DriverError{Code: ErrorUnsupportedInvocationMode})
	}
	if g.attempt != nil {
		return encodeTextAttemptOpenResponse(nil, attemptStateError())
	}
	result, driverError := g.driver.OpenTextAttempt(request)
	if invalidResult(result, driverError) {
		closeOpenResult(result)
		return encodeTextAttemptOpenResponse(nil, internalError())
	}
	if driverError != nil {
		return encodeTextAttemptOpenResponse(nil, driverError)
	}
	if result.Attempt == nil || (result.Request == nil) == (result.Response == nil) ||
		result.Request != nil && !validRequestPlan(result.Request) ||
		result.Response != nil && !validClientResponse(result.Response, true) {
		closeOpenResult(result)
		return encodeTextAttemptOpenResponse(nil, internalError())
	}
	handle := g.nextAttemptHandle()
	g.attempt = result.Attempt
	g.handle = handle
	g.mode = request.Invocation.Mode
	g.phase = attemptPhaseReady
	g.sawOutcome = result.Response != nil && result.Response.Outcome != nil
	g.sawUsage = result.Response != nil && result.Response.Usage != nil
	return encodeTextAttemptOpenResponse(&TextAttemptOpenSuccess{
		AttemptHandle: handle, Request: result.Request, Response: result.Response,
	}, nil)
}

func (g *Guest) HandleTextTransformBufferedResponse(input []byte) []byte {
	request, err := decodeTextTransformBufferedResponseRequest(input)
	if err != nil || request.Upstream == nil {
		return encodeTextTransformBufferedResponseResponse(nil, internalError())
	}
	attempt, driverError := g.attemptInPhase(request.AttemptHandle, attemptPhaseReady, TextModeBuffered)
	if driverError != nil {
		return encodeTextTransformBufferedResponseResponse(nil, driverError)
	}
	g.phase = attemptPhaseTerminal
	success, driverError := attempt.TransformBufferedResponse(*request.Upstream)
	if invalidResult(success, driverError) || driverError != nil && !validResponseHookError(driverError) ||
		driverError == nil && (success.Response == nil || !validClientResponse(success.Response, true)) {
		return encodeTextTransformBufferedResponseResponse(nil, internalError())
	}
	return encodeTextTransformBufferedResponseResponse(success, driverError)
}

func (g *Guest) HandleTextSSEOpen(input []byte) []byte {
	request, err := decodeTextSSEOpenRequest(input)
	if err != nil || request.Upstream == nil {
		return encodeTextSSEOpenResponse(nil, internalError())
	}
	attempt, driverError := g.attemptInPhase(request.AttemptHandle, attemptPhaseReady, TextModeSSE)
	if driverError != nil {
		return encodeTextSSEOpenResponse(nil, driverError)
	}
	g.phase = attemptPhaseTerminal
	success, driverError := attempt.OpenSSE(*request.Upstream)
	if invalidResult(success, driverError) || driverError != nil && !validResponseHookError(driverError) ||
		driverError == nil && !validSSEOpen(success) {
		return encodeTextSSEOpenResponse(nil, internalError())
	}
	if driverError == nil {
		g.phase = attemptPhaseSSEOpen
		g.sawOutcome = success.Outcome != nil
	}
	return encodeTextSSEOpenResponse(success, driverError)
}

func (g *Guest) HandleTextSSETransformEvent(input []byte) []byte {
	request, err := decodeTextSSETransformEventRequest(input)
	if err != nil || request.Upstream == nil {
		return encodeTextSSETransformEventResponse(nil, internalError())
	}
	attempt, driverError := g.attemptInPhase(request.AttemptHandle, attemptPhaseSSEOpen, TextModeSSE)
	if driverError != nil {
		return encodeTextSSETransformEventResponse(nil, driverError)
	}
	success, driverError := attempt.TransformSSEEvent(*request.Upstream)
	if invalidResult(success, driverError) || driverError != nil && !validResponseHookError(driverError) ||
		driverError == nil && !validStreamResult(success.Events, success.Outcome, success.Usage) {
		g.phase = attemptPhaseTerminal
		return encodeTextSSETransformEventResponse(nil, internalError())
	}
	if driverError != nil {
		g.phase = attemptPhaseTerminal
	} else {
		g.sawOutcome = g.sawOutcome || success.Outcome != nil
		g.sawUsage = g.sawUsage || success.Usage != nil
	}
	return encodeTextSSETransformEventResponse(success, driverError)
}

func (g *Guest) HandleTextSSEFinish(input []byte) []byte {
	request, err := decodeTextSSEFinishRequest(input)
	if err != nil {
		return encodeTextSSEFinishResponse(nil, internalError())
	}
	attempt, driverError := g.attemptInPhase(request.AttemptHandle, attemptPhaseSSEOpen, TextModeSSE)
	if driverError != nil {
		return encodeTextSSEFinishResponse(nil, driverError)
	}
	g.phase = attemptPhaseTerminal
	success, driverError := attempt.FinishSSE()
	if invalidResult(success, driverError) || driverError != nil && !validResponseHookError(driverError) ||
		driverError == nil && (!validStreamResult(success.Events, success.Outcome, success.Usage) ||
			!g.sawOutcome && success.Outcome == nil || !g.sawUsage && success.Usage == nil) {
		return encodeTextSSEFinishResponse(nil, internalError())
	}
	return encodeTextSSEFinishResponse(success, driverError)
}

func (g *Guest) HandleTextAttemptClose(input []byte) []byte {
	request, err := decodeTextAttemptCloseRequest(input)
	if err != nil {
		return encodeTextAttemptCloseResponse(nil, internalError())
	}
	attempt, driverError := g.activeAttempt(request.AttemptHandle)
	if driverError != nil {
		return encodeTextAttemptCloseResponse(nil, driverError)
	}
	g.attempt = nil
	g.handle = 0
	g.mode = TextModeUnspecified
	g.phase = attemptPhaseNone
	g.sawOutcome = false
	g.sawUsage = false
	if driverError = attempt.Close(); driverError != nil {
		return encodeTextAttemptCloseResponse(nil, driverError)
	}
	return encodeTextAttemptCloseResponse(&TextAttemptCloseSuccess{}, nil)
}

func closeOpenResult(result *TextAttemptOpenResult) {
	if result != nil && result.Attempt != nil {
		_ = result.Attempt.Close()
	}
}

func (g *Guest) activeAttempt(handle uint64) (TextAttempt, *DriverError) {
	if g == nil || handle == 0 || g.attempt == nil || handle != g.handle {
		return nil, &DriverError{Code: ErrorInvalidAttempt}
	}
	return g.attempt, nil
}

func (g *Guest) attemptInPhase(handle uint64, phase attemptPhase, mode TextMode) (TextAttempt, *DriverError) {
	attempt, driverError := g.activeAttempt(handle)
	if driverError != nil {
		return nil, driverError
	}
	if g.phase != phase || mode != TextModeUnspecified && g.mode != mode {
		return nil, attemptStateError()
	}
	return attempt, nil
}

func (g *Guest) nextAttemptHandle() uint64 {
	handle := g.nextHandle
	g.nextHandle++
	if g.nextHandle == 0 {
		g.nextHandle = 1
	}
	return handle
}

func invalidResult[T any](success *T, failure *DriverError) bool {
	return (success == nil) == (failure == nil)
}

func internalError() *DriverError     { return &DriverError{Code: ErrorDriverInternal} }
func attemptStateError() *DriverError { return &DriverError{Code: ErrorInvalidAttemptState} }

func validInvocation(value *TextInvocation) bool {
	return value != nil && value.Request != nil && validProtocolPayload(value.Request) &&
		(value.ProtocolMetadata == nil || validProtocolPayload(value.ProtocolMetadata)) &&
		value.SelectedUpstreamModel != "" && value.ResponseID != ""
}

func validProtocolPayload(value *ProtocolPayload) bool {
	return value != nil && validContract(value.ProtocolContract) && value.MediaType == MediaTypeJSON && len(value.JSON) != 0
}

func validContract(value string) bool {
	switch value {
	case ProtocolContractOpenAIChatCompletions20260718, ProtocolContractOpenAIResponses20260718, ProtocolContractAnthropicMessages20260718:
		return true
	default:
		return false
	}
}

func validTextCapabilities(value *TextCapabilities) bool {
	if value == nil || len(value.ProtocolContracts) == 0 || !sort.StringsAreSorted(value.ProtocolContracts) {
		return false
	}
	for index, contract := range value.ProtocolContracts {
		if !validContract(contract) || index > 0 && contract == value.ProtocolContracts[index-1] {
			return false
		}
	}
	return true
}

func validRequestPlan(value *RequestPlan) bool {
	return value != nil && value.EndpointRef != "" && value.Method != "" && value.RelativePath != "" &&
		(value.Body == nil) != (value.BodyPlan == nil) && value.Auth != nil
}

func validClientResponse(value *ClientResponse, requireUsage bool) bool {
	return value != nil && value.StatusCode >= 200 && value.StatusCode <= 599 && validProtocolPayload(value.Body) &&
		(value.Outcome == nil || validOutcome(value.Outcome)) && (!requireUsage || value.Usage != nil) &&
		(value.Usage == nil || validUsageReport(value.Usage))
}

func validSSEOpen(value *TextSSEOpenSuccess) bool {
	return value != nil && value.StatusCode >= 200 && value.StatusCode <= 599 && (value.Outcome == nil || validOutcome(value.Outcome))
}

func validStreamResult(events []ProtocolEventPayload, outcome *SemanticOutcome, usage *UsageReport) bool {
	for index := range events {
		if !validContract(events[index].ProtocolContract) || events[index].EventType == "" || len(events[index].JSON) == 0 {
			return false
		}
	}
	return (outcome == nil || validOutcome(outcome)) && (usage == nil || validUsageReport(usage))
}

func validOutcome(value *SemanticOutcome) bool {
	if value == nil || !validVendorCode(value.VendorCode) {
		return false
	}
	switch value.Class {
	case SemanticOutcomeClassSuccess:
		return value.VendorCode == ""
	case SemanticOutcomeClassCallerError, SemanticOutcomeClassEndpointError, SemanticOutcomeClassMappingError:
		return true
	default:
		return false
	}
}

func validVendorCode(value string) bool {
	if len(value) > 256 || !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func validUsageReport(value *UsageReport) bool {
	if value == nil || value.Provenance < UsageProvenanceUpstreamReported || value.Provenance > UsageProvenanceDriverAccumulated {
		return false
	}
	counts := []*int64{value.InputTokens, value.OutputTokens, value.CachedTokens, value.ReasoningTokens}
	known := 0
	for _, count := range counts {
		if count != nil {
			known++
			if *count < 0 {
				return false
			}
		}
	}
	switch value.Status {
	case UsageStatusFinal:
		if value.InputTokens == nil || value.OutputTokens == nil {
			return false
		}
	case UsageStatusPartial:
		if known == 0 {
			return false
		}
	case UsageStatusUnavailable:
		if known != 0 {
			return false
		}
	default:
		return false
	}
	if value.InputTokens != nil && value.CachedTokens != nil && *value.CachedTokens > *value.InputTokens ||
		value.OutputTokens != nil && value.ReasoningTokens != nil && *value.ReasoningTokens > *value.OutputTokens {
		return false
	}
	return value.InputTokens == nil || value.OutputTokens == nil || *value.InputTokens <= math.MaxInt64-*value.OutputTokens
}

func validResponseHookError(value *DriverError) bool {
	return value == nil || value.Usage == nil || validUsageReport(value.Usage) &&
		(value.Usage.Status == UsageStatusPartial || value.Usage.Status == UsageStatusUnavailable)
}
