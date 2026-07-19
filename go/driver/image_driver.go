package driver

import "sort"

type ImageDriver interface {
	Bind(BindRequest) (*BindSuccess, *DriverError)
	OpenImageAttempt(ImageAttemptOpenRequest) (*ImageAttemptOpenResult, *DriverError)
}

type ImageAttemptOpenResult struct {
	Attempt  ImageAttempt
	Request  *RequestPlan
	Response *ImageClientResponse
}

type ImageAttempt interface {
	TransformBufferedResponse(ImageUpstreamResponse) (*ImageTransformBufferedResponseSuccess, *DriverError)
	Close() *DriverError
}

type ImageGuest struct {
	driver     ImageDriver
	attempt    ImageAttempt
	handle     uint64
	nextHandle uint64
	contract   string
	phase      attemptPhase
}

func NewImageGuest(imageDriver ImageDriver) *ImageGuest {
	return &ImageGuest{driver: imageDriver, nextHandle: 1}
}

func (g *ImageGuest) HandleBind(input []byte) []byte {
	request, err := decodeBindRequest(input)
	if err != nil || g == nil || g.driver == nil {
		return encodeBindResponse(nil, internalError())
	}
	success, driverError := g.driver.Bind(request)
	if invalidResult(success, driverError) || success != nil &&
		(!validImageCapabilities(success.ImageCapabilities) || success.TextCapabilities != nil) {
		return encodeBindResponse(nil, internalError())
	}
	return encodeBindResponse(success, driverError)
}

func (g *ImageGuest) HandleImageAttemptOpen(input []byte) []byte {
	request, err := decodeImageAttemptOpenRequest(input)
	if err != nil || g == nil || g.driver == nil {
		return encodeImageAttemptOpenResponse(nil, internalError())
	}
	if !validImageInvocation(request.Invocation) {
		return encodeImageAttemptOpenResponse(nil, &DriverError{Code: ErrorInvalidInvocation})
	}
	if g.attempt != nil {
		return encodeImageAttemptOpenResponse(nil, attemptStateError())
	}
	result, driverError := g.driver.OpenImageAttempt(request)
	if invalidResult(result, driverError) {
		closeImageOpenResult(result)
		return encodeImageAttemptOpenResponse(nil, internalError())
	}
	if driverError != nil {
		return encodeImageAttemptOpenResponse(nil, driverError)
	}
	if result.Attempt == nil || (result.Request == nil) == (result.Response == nil) ||
		result.Request != nil && !validRequestPlan(result.Request) ||
		result.Response != nil && !validImageClientResponse(result.Response, request.Invocation.Request.ProtocolContract) {
		closeImageOpenResult(result)
		return encodeImageAttemptOpenResponse(nil, internalError())
	}
	handle := g.nextAttemptHandle()
	g.attempt = result.Attempt
	g.handle = handle
	g.contract = request.Invocation.Request.ProtocolContract
	g.phase = attemptPhaseReady
	return encodeImageAttemptOpenResponse(&ImageAttemptOpenSuccess{
		AttemptHandle: handle, Request: result.Request, Response: result.Response,
	}, nil)
}

func (g *ImageGuest) HandleImageTransformBufferedResponse(input []byte) []byte {
	request, err := decodeImageTransformBufferedResponseRequest(input)
	if err != nil || request.Upstream == nil || !validImageUpstreamResponse(request.Upstream) {
		return encodeImageTransformBufferedResponseResponse(nil, internalError())
	}
	attempt, driverError := g.attemptInPhase(request.AttemptHandle)
	if driverError != nil {
		return encodeImageTransformBufferedResponseResponse(nil, driverError)
	}
	g.phase = attemptPhaseTerminal
	success, driverError := attempt.TransformBufferedResponse(*request.Upstream)
	if invalidResult(success, driverError) || driverError != nil && !validResponseHookError(driverError) ||
		driverError == nil && (success.Response == nil || !validImageClientResponse(success.Response, g.contract)) {
		return encodeImageTransformBufferedResponseResponse(nil, internalError())
	}
	return encodeImageTransformBufferedResponseResponse(success, driverError)
}

func (g *ImageGuest) HandleImageAttemptClose(input []byte) []byte {
	request, err := decodeImageAttemptCloseRequest(input)
	if err != nil {
		return encodeImageAttemptCloseResponse(nil, internalError())
	}
	attempt, driverError := g.activeAttempt(request.AttemptHandle)
	if driverError != nil {
		return encodeImageAttemptCloseResponse(nil, driverError)
	}
	g.attempt = nil
	g.handle = 0
	g.contract = ""
	g.phase = attemptPhaseNone
	if driverError = attempt.Close(); driverError != nil {
		return encodeImageAttemptCloseResponse(nil, driverError)
	}
	return encodeImageAttemptCloseResponse(&ImageAttemptCloseSuccess{}, nil)
}

func (g *ImageGuest) activeAttempt(handle uint64) (ImageAttempt, *DriverError) {
	if g == nil || handle == 0 || g.attempt == nil || handle != g.handle {
		return nil, &DriverError{Code: ErrorInvalidAttempt}
	}
	return g.attempt, nil
}

func (g *ImageGuest) attemptInPhase(handle uint64) (ImageAttempt, *DriverError) {
	attempt, driverError := g.activeAttempt(handle)
	if driverError != nil {
		return nil, driverError
	}
	if g.phase != attemptPhaseReady {
		return nil, attemptStateError()
	}
	return attempt, nil
}

func (g *ImageGuest) nextAttemptHandle() uint64 {
	handle := g.nextHandle
	g.nextHandle++
	if g.nextHandle == 0 {
		g.nextHandle = 1
	}
	return handle
}

func closeImageOpenResult(result *ImageAttemptOpenResult) {
	if result != nil && result.Attempt != nil {
		_ = result.Attempt.Close()
	}
}

func validImageCapabilities(value *ImageCapabilities) bool {
	if value == nil || len(value.ProtocolContracts) == 0 || !sort.StringsAreSorted(value.ProtocolContracts) {
		return false
	}
	for index, contract := range value.ProtocolContracts {
		if !validImageContract(contract) || index > 0 && contract == value.ProtocolContracts[index-1] {
			return false
		}
	}
	return true
}

func validImageContract(contract string) bool {
	return contract == ProtocolContractOpenAIImageGenerations20260719 || contract == ProtocolContractOpenAIImageEdits20260719
}

func validImageInvocation(value *ImageInvocation) bool {
	if value == nil || value.Request == nil || !validImageContract(value.Request.ProtocolContract) ||
		value.SelectedUpstreamModel == "" || value.ResponseID == "" {
		return false
	}
	request := value.Request
	if request.ProtocolContract == ProtocolContractOpenAIImageGenerations20260719 {
		return request.MediaType == MediaTypeJSON && len(request.JSON) != 0 && request.Multipart == nil
	}
	return request.Multipart != nil && request.JSON == nil && len(request.Multipart.Parts) != 0
}

func validImageUpstreamResponse(value *ImageUpstreamResponse) bool {
	return value != nil && value.Head != nil && value.Body != nil && value.Body.MediaType != "" &&
		(value.Body.Inline == nil) != (value.Body.Blob == nil) &&
		(value.Body.Blob == nil || validBlobRef(*value.Body.Blob))
}

func validImageClientResponse(value *ImageClientResponse, contract string) bool {
	return value != nil && value.StatusCode >= 200 && value.StatusCode <= 599 &&
		value.ProtocolContract == contract && validBodyPlan(value.Body) &&
		(value.Outcome == nil || validOutcome(value.Outcome)) && validUsageReport(value.Usage)
}

func validBodyPlan(value *BodyPlan) bool {
	if value == nil || value.MediaType == "" {
		return false
	}
	switch value.Kind {
	case BodyPlanInline:
		return value.Blob == nil && value.Segments == nil && value.Parts == nil
	case BodyPlanBlob, BodyPlanBase64:
		return value.Blob != nil && validBlobRef(*value.Blob) && value.Inline == nil && value.Segments == nil && value.Parts == nil
	case BodyPlanComposite:
		if len(value.Segments) == 0 || value.Inline != nil || value.Blob != nil || value.Parts != nil {
			return false
		}
		for _, segment := range value.Segments {
			if !validBodySegment(segment) {
				return false
			}
		}
		return true
	case BodyPlanMultipart:
		if len(value.Parts) == 0 || value.Inline != nil || value.Blob != nil || value.Segments != nil {
			return false
		}
		for _, part := range value.Parts {
			if part.Name == "" || !validBodySegment(part.Content) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validBodySegment(value BodySegment) bool {
	switch value.Kind {
	case BodySourceInline:
		return value.Blob == nil
	case BodySourceBlob, BodySourceBase64:
		return value.Blob != nil && validBlobRef(*value.Blob) && value.Inline == nil
	default:
		return false
	}
}

func validBlobRef(value BlobRef) bool { return value.ID != 0 && value.Size >= 0 }
