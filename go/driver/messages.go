// Package driver defines the protocol-neutral Legate Driver ABI v1 guest contract.
package driver

const (
	ProtocolContractOpenAIChatCompletions20260718  = "openai.chat_completions/2026-07-18"
	ProtocolContractOpenAIResponses20260718        = "openai.responses/2026-07-18"
	ProtocolContractAnthropicMessages20260718      = "anthropic.messages/2026-07-18"
	ProtocolContractOpenAIImageGenerations20260719 = "openai.images.generations/2026-07-19"
	ProtocolContractOpenAIImageEdits20260719       = "openai.images.edits/2026-07-19"
	MediaTypeJSON                                  = "application/json"

	ErrorInvalidConfig             = "invalid_config"
	ErrorUnsupportedFeature        = "unsupported_feature"
	ErrorUnsupportedInvocationMode = "unsupported_invocation_mode"
	ErrorInvalidProtocolRequest    = "protocol_request_invalid"
	ErrorInvalidProtocolResponse   = "protocol_response_invalid"
	ErrorInvalidInvocation         = "invalid_invocation"
	ErrorMalformedUpstreamResponse = "malformed_upstream_response"
	ErrorUpstreamStreamTruncated   = "upstream_stream_truncated"
	ErrorInvalidAttempt            = "invalid_attempt"
	ErrorInvalidAttemptState       = "invalid_attempt_state"
	ErrorDriverInternal            = "driver_internal"
	ErrorDriverResourceExhausted   = "driver_resource_exhausted"
)

type TextMode int32

const (
	TextModeUnspecified TextMode = iota
	TextModeBuffered
	TextModeSSE
)

type AuthKind int32

const (
	AuthKindUnspecified AuthKind = iota
	AuthKindNone
	AuthKindBearer
	AuthKindAPIKeyHeader
)

type SemanticOutcomeClass int32

const (
	SemanticOutcomeClassUnspecified SemanticOutcomeClass = iota
	SemanticOutcomeClassSuccess
	SemanticOutcomeClassCallerError
	SemanticOutcomeClassEndpointError
	SemanticOutcomeClassMappingError
)

type UsageStatus int32

const (
	UsageStatusUnspecified UsageStatus = iota
	UsageStatusFinal
	UsageStatusPartial
	UsageStatusUnavailable
)

type UsageProvenance int32

const (
	UsageProvenanceUnspecified UsageProvenance = iota
	UsageProvenanceUpstreamReported
	UsageProvenanceDriverAccumulated
)

type FieldIssue struct {
	Pointer string
	Code    string
	Message string
}

// DriverError contains only stable, secret-safe diagnostics. Protocol-level
// rejections are ClientResponse values, not DriverError values.
type DriverError struct {
	Code    string
	Message string
	Issues  []FieldIssue
	Usage   *UsageReport
}

type NameValues struct {
	Name   string
	Values []string
}

type MessageBody struct {
	MediaType string
	Payload   []byte
}

// ProtocolPayload contains one bounded JSON document for one exact contract.
type ProtocolPayload struct {
	ProtocolContract string
	MediaType        string
	JSON             []byte
}

// ProtocolEventPayload contains one typed protocol event. Core owns SSE framing.
type ProtocolEventPayload struct {
	ProtocolContract string
	EventType        string
	JSON             []byte
}

type CredentialSlotDescriptor struct {
	Name       string
	Configured bool
}

type TextCapabilities struct {
	ProtocolContracts []string
}

type ImageCapabilities struct {
	ProtocolContracts []string
}

type BindRequest struct {
	ConfigJSON      []byte
	EndpointRefs    []string
	CredentialSlots []CredentialSlotDescriptor
}

type BindSuccess struct {
	BoundState        []byte
	TextCapabilities  *TextCapabilities
	ImageCapabilities *ImageCapabilities
}

type AuthPlan struct {
	Kind           AuthKind
	CredentialSlot string
	HeaderName     string
}

type RequestPlan struct {
	EndpointRef  string
	Method       string
	RelativePath string
	Query        []NameValues
	Headers      []NameValues
	Body         *MessageBody
	Auth         *AuthPlan
	BodyPlan     *BodyPlan
}

type UsageReport struct {
	Status          UsageStatus
	InputTokens     *int64
	OutputTokens    *int64
	CachedTokens    *int64
	ReasoningTokens *int64
	Provenance      UsageProvenance
}

type SemanticOutcome struct {
	Class      SemanticOutcomeClass
	VendorCode string
}

type ClientResponse struct {
	StatusCode int32
	Headers    []NameValues
	Body       *ProtocolPayload
	Outcome    *SemanticOutcome
	Usage      *UsageReport
}

// Response builds a protocol-native client response. Attach Usage before
// returning it. Outcome is optional; Core derives the default class from
// authoritative status information.
func Response(statusCode int32, headers []NameValues, body ProtocolPayload) *ClientResponse {
	return &ClientResponse{StatusCode: statusCode, Headers: headers, Body: &body}
}

func (r *ClientResponse) WithOutcome(outcome SemanticOutcome) *ClientResponse {
	if r != nil {
		r.Outcome = &outcome
	}
	return r
}

func (r *ClientResponse) WithUsage(usage UsageReport) *ClientResponse {
	if r != nil {
		r.Usage = &usage
	}
	return r
}

func Success() SemanticOutcome { return SemanticOutcome{Class: SemanticOutcomeClassSuccess} }
func CallerError(code string) SemanticOutcome {
	return SemanticOutcome{Class: SemanticOutcomeClassCallerError, VendorCode: code}
}
func EndpointError(code string) SemanticOutcome {
	return SemanticOutcome{Class: SemanticOutcomeClassEndpointError, VendorCode: code}
}
func MappingError(code string) SemanticOutcome {
	return SemanticOutcome{Class: SemanticOutcomeClassMappingError, VendorCode: code}
}

func UnavailableUsage() UsageReport {
	return UsageReport{Status: UsageStatusUnavailable, Provenance: UsageProvenanceUpstreamReported}
}

type TextInvocation struct {
	Mode                  TextMode
	Request               *ProtocolPayload
	ProtocolMetadata      *ProtocolPayload
	SelectedUpstreamModel string
	ResponseID            string
}

type TextAttemptOpenRequest struct {
	BoundState []byte
	Invocation *TextInvocation
}

// Exactly one of Request and Response must be set.
type TextAttemptOpenSuccess struct {
	AttemptHandle uint64
	Request       *RequestPlan
	Response      *ClientResponse
}

type UpstreamResponseHead struct {
	StatusCode int32
	Headers    []NameValues
}

type BufferedUpstreamResponse struct {
	Head *UpstreamResponseHead
	Body *MessageBody
}

type TextTransformBufferedResponseRequest struct {
	AttemptHandle uint64
	Upstream      *BufferedUpstreamResponse
}

type TextTransformBufferedResponseSuccess struct {
	Response *ClientResponse
}

type TextSSEOpenSuccess struct {
	StatusCode int32
	Headers    []NameValues
	Outcome    *SemanticOutcome
}

type TextSSEOpenRequest struct {
	AttemptHandle uint64
	Upstream      *UpstreamResponseHead
}

type UpstreamSSEEvent struct {
	EventType         string
	Data              []byte
	LastEventID       *string
	RetryMilliseconds *uint64
}

type TextSSETransformEventRequest struct {
	AttemptHandle uint64
	Upstream      *UpstreamSSEEvent
}

type TextSSETransformEventSuccess struct {
	Events  []ProtocolEventPayload
	Outcome *SemanticOutcome
	Usage   *UsageReport
}

type TextSSEFinishSuccess struct {
	Events  []ProtocolEventPayload
	Outcome *SemanticOutcome
	Usage   *UsageReport
}

type TextSSEFinishRequest struct {
	AttemptHandle uint64
}

type TextAttemptCloseSuccess struct{}

type TextAttemptCloseRequest struct {
	AttemptHandle uint64
}

type BlobRef struct {
	ID     uint64
	Size   int64
	SHA256 [32]byte
}

type MultipartInputPart struct {
	Name        string
	Filename    *string
	Headers     []NameValues
	ContentType string
	Inline      []byte
	Blob        *BlobRef
}

type MultipartInput struct {
	Parts []MultipartInputPart
}

type ImageProtocolRequest struct {
	ProtocolContract string
	MediaType        string
	JSON             []byte
	Multipart        *MultipartInput
}

type ImageInvocation struct {
	Request               *ImageProtocolRequest
	SelectedUpstreamModel string
	ResponseID            string
	Blobs                 []BlobRef
}

type BodySourceKind uint8

const (
	BodySourceUnspecified BodySourceKind = iota
	BodySourceInline
	BodySourceBlob
	BodySourceBase64
)

type BodySegment struct {
	Kind   BodySourceKind
	Inline []byte
	Blob   *BlobRef
}

type MultipartBodyPart struct {
	Name        string
	Filename    *string
	Headers     []NameValues
	ContentType string
	Content     BodySegment
}

type BodyPlanKind uint8

const (
	BodyPlanUnspecified BodyPlanKind = iota
	BodyPlanInline
	BodyPlanBlob
	BodyPlanBase64
	BodyPlanComposite
	BodyPlanMultipart
)

type BodyPlan struct {
	Kind      BodyPlanKind
	MediaType string
	Inline    []byte
	Blob      *BlobRef
	Segments  []BodySegment
	Parts     []MultipartBodyPart
}

func InlineBody(mediaType string, value []byte) *BodyPlan {
	return &BodyPlan{Kind: BodyPlanInline, MediaType: mediaType, Inline: append([]byte(nil), value...)}
}

func BlobBody(mediaType string, ref BlobRef) *BodyPlan {
	return &BodyPlan{Kind: BodyPlanBlob, MediaType: mediaType, Blob: cloneBlobRef(ref)}
}

func Base64Body(mediaType string, ref BlobRef) *BodyPlan {
	return &BodyPlan{Kind: BodyPlanBase64, MediaType: mediaType, Blob: cloneBlobRef(ref)}
}

func NewCompositeBody(mediaType string) *BodyPlan {
	return &BodyPlan{Kind: BodyPlanComposite, MediaType: mediaType}
}

func NewMultipartBody(mediaType string) *BodyPlan {
	return &BodyPlan{Kind: BodyPlanMultipart, MediaType: mediaType}
}

func (p *BodyPlan) AddInline(value []byte) *BodyPlan {
	if p != nil && p.Kind == BodyPlanComposite {
		p.Segments = append(p.Segments, BodySegment{Kind: BodySourceInline, Inline: append([]byte(nil), value...)})
	}
	return p
}

func (p *BodyPlan) AddBlob(ref BlobRef) *BodyPlan {
	if p != nil && p.Kind == BodyPlanComposite {
		p.Segments = append(p.Segments, BodySegment{Kind: BodySourceBlob, Blob: cloneBlobRef(ref)})
	}
	return p
}

func (p *BodyPlan) AddBase64(ref BlobRef) *BodyPlan {
	if p != nil && p.Kind == BodyPlanComposite {
		p.Segments = append(p.Segments, BodySegment{Kind: BodySourceBase64, Blob: cloneBlobRef(ref)})
	}
	return p
}

func (p *BodyPlan) AddPart(part MultipartBodyPart) *BodyPlan {
	if p != nil && p.Kind == BodyPlanMultipart {
		p.Parts = append(p.Parts, part)
	}
	return p
}

func InlineSegment(value []byte) BodySegment {
	return BodySegment{Kind: BodySourceInline, Inline: append([]byte(nil), value...)}
}

func BlobSegment(ref BlobRef) BodySegment {
	return BodySegment{Kind: BodySourceBlob, Blob: cloneBlobRef(ref)}
}

func Base64Segment(ref BlobRef) BodySegment {
	return BodySegment{Kind: BodySourceBase64, Blob: cloneBlobRef(ref)}
}

func cloneBlobRef(ref BlobRef) *BlobRef {
	clone := ref
	return &clone
}

type ImageAttemptOpenRequest struct {
	BoundState []byte
	Invocation *ImageInvocation
}

type ImageAttemptOpenSuccess struct {
	AttemptHandle uint64
	Request       *RequestPlan
	Response      *ImageClientResponse
}

type ImageUpstreamBody struct {
	MediaType string
	Inline    []byte
	Blob      *BlobRef
}

type ImageUpstreamResponse struct {
	Head *UpstreamResponseHead
	Body *ImageUpstreamBody
}

type ImageClientResponse struct {
	StatusCode       int32
	Headers          []NameValues
	ProtocolContract string
	Body             *BodyPlan
	Outcome          *SemanticOutcome
	Usage            *UsageReport
}

func ImageResponse(statusCode int32, contract string, headers []NameValues, body *BodyPlan) *ImageClientResponse {
	return &ImageClientResponse{StatusCode: statusCode, ProtocolContract: contract, Headers: headers, Body: body}
}

func (r *ImageClientResponse) WithOutcome(outcome SemanticOutcome) *ImageClientResponse {
	if r != nil {
		r.Outcome = &outcome
	}
	return r
}

func (r *ImageClientResponse) WithUsage(usage UsageReport) *ImageClientResponse {
	if r != nil {
		r.Usage = &usage
	}
	return r
}

type ImageTransformBufferedResponseRequest struct {
	AttemptHandle uint64
	Upstream      *ImageUpstreamResponse
}

type ImageTransformBufferedResponseSuccess struct {
	Response *ImageClientResponse
}

type ImageAttemptCloseRequest struct {
	AttemptHandle uint64
}

type ImageAttemptCloseSuccess struct{}
