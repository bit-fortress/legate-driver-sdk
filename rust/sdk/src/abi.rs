//! Protobuf messages for Legate Driver ABI v1.
#![allow(clippy::large_enum_variant)]

#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum TextMode {
    Unspecified = 0,
    Buffered = 1,
    Sse = 2,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum AuthKind {
    Unspecified = 0,
    None = 1,
    Bearer = 2,
    ApiKeyHeader = 3,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum SemanticOutcomeClass {
    Unspecified = 0,
    Success = 1,
    CallerError = 2,
    EndpointError = 3,
    MappingError = 4,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum UsageStatus {
    Unspecified = 0,
    Final = 1,
    Partial = 2,
    Unavailable = 3,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum UsageProvenance {
    Unspecified = 0,
    UpstreamReported = 1,
    DriverAccumulated = 2,
}

#[derive(Clone, PartialEq, ::prost::Message)]
pub struct FieldIssue {
    #[prost(string, tag = "1")]
    pub pointer: String,
    #[prost(string, tag = "2")]
    pub code: String,
    #[prost(string, tag = "3")]
    pub message: String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct DriverError {
    #[prost(string, tag = "1")]
    pub code: String,
    #[prost(string, tag = "2")]
    pub message: String,
    #[prost(message, repeated, tag = "3")]
    pub issues: Vec<FieldIssue>,
    #[prost(message, optional, tag = "4")]
    pub usage: Option<UsageReport>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct NameValues {
    #[prost(string, tag = "1")]
    pub name: String,
    #[prost(string, repeated, tag = "2")]
    pub values: Vec<String>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MessageBody {
    #[prost(string, tag = "1")]
    pub media_type: String,
    #[prost(bytes = "vec", tag = "2")]
    pub payload: Vec<u8>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ProtocolPayload {
    #[prost(string, tag = "1")]
    pub protocol_contract: String,
    #[prost(string, tag = "2")]
    pub media_type: String,
    #[prost(bytes = "vec", tag = "3")]
    pub json: Vec<u8>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ProtocolEventPayload {
    #[prost(string, tag = "1")]
    pub protocol_contract: String,
    #[prost(string, tag = "2")]
    pub event_type: String,
    #[prost(bytes = "vec", tag = "3")]
    pub json: Vec<u8>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct CredentialSlotDescriptor {
    #[prost(string, tag = "1")]
    pub name: String,
    #[prost(bool, tag = "2")]
    pub configured: bool,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextCapabilities {
    #[prost(string, repeated, tag = "1")]
    pub protocol_contracts: Vec<String>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageCapabilities {
    #[prost(string, repeated, tag = "1")]
    pub protocol_contracts: Vec<String>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BindRequest {
    #[prost(bytes = "vec", tag = "1")]
    pub config_json: Vec<u8>,
    #[prost(string, repeated, tag = "2")]
    pub endpoint_refs: Vec<String>,
    #[prost(message, repeated, tag = "3")]
    pub credential_slots: Vec<CredentialSlotDescriptor>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BindSuccess {
    #[prost(bytes = "vec", tag = "1")]
    pub bound_state: Vec<u8>,
    #[prost(oneof = "bind_success::Capabilities", tags = "2, 3")]
    pub capabilities: Option<bind_success::Capabilities>,
}
pub mod bind_success {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Capabilities {
        #[prost(message, tag = "2")]
        TextCapabilities(super::TextCapabilities),
        #[prost(message, tag = "3")]
        ImageCapabilities(super::ImageCapabilities),
    }
}

#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BindResponse {
    #[prost(oneof = "bind_response::Result", tags = "1, 2")]
    pub result: Option<bind_response::Result>,
}
pub mod bind_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::BindSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}

#[derive(Clone, PartialEq, ::prost::Message)]
pub struct AuthPlan {
    #[prost(enumeration = "AuthKind", tag = "1")]
    pub kind: i32,
    #[prost(string, tag = "2")]
    pub credential_slot: String,
    #[prost(string, tag = "3")]
    pub header_name: String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct RequestPlan {
    #[prost(string, tag = "1")]
    pub endpoint_ref: String,
    #[prost(string, tag = "2")]
    pub method: String,
    #[prost(string, tag = "3")]
    pub relative_path: String,
    #[prost(message, repeated, tag = "4")]
    pub query: Vec<NameValues>,
    #[prost(message, repeated, tag = "5")]
    pub headers: Vec<NameValues>,
    #[prost(message, optional, tag = "6")]
    pub body: Option<MessageBody>,
    #[prost(message, optional, tag = "7")]
    pub auth: Option<AuthPlan>,
    #[prost(message, optional, tag = "8")]
    pub body_plan: Option<BodyPlan>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct UsageReport {
    #[prost(enumeration = "UsageStatus", tag = "1")]
    pub status: i32,
    #[prost(int64, optional, tag = "2")]
    pub input_tokens: Option<i64>,
    #[prost(int64, optional, tag = "3")]
    pub output_tokens: Option<i64>,
    #[prost(int64, optional, tag = "4")]
    pub cached_tokens: Option<i64>,
    #[prost(int64, optional, tag = "5")]
    pub reasoning_tokens: Option<i64>,
    #[prost(enumeration = "UsageProvenance", tag = "6")]
    pub provenance: i32,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct SemanticOutcome {
    #[prost(enumeration = "SemanticOutcomeClass", tag = "1")]
    pub class: i32,
    #[prost(string, tag = "2")]
    pub vendor_code: String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ClientResponse {
    #[prost(int32, tag = "1")]
    pub status_code: i32,
    #[prost(message, repeated, tag = "2")]
    pub headers: Vec<NameValues>,
    #[prost(message, optional, tag = "3")]
    pub body: Option<ProtocolPayload>,
    #[prost(message, optional, tag = "4")]
    pub outcome: Option<SemanticOutcome>,
    #[prost(message, optional, tag = "5")]
    pub usage: Option<UsageReport>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextInvocation {
    #[prost(enumeration = "TextMode", tag = "1")]
    pub mode: i32,
    #[prost(message, optional, tag = "2")]
    pub request: Option<ProtocolPayload>,
    #[prost(message, optional, tag = "3")]
    pub protocol_metadata: Option<ProtocolPayload>,
    #[prost(string, tag = "4")]
    pub selected_upstream_model: String,
    #[prost(string, tag = "5")]
    pub response_id: String,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextAttemptOpenRequest {
    #[prost(bytes = "vec", tag = "1")]
    pub bound_state: Vec<u8>,
    #[prost(message, optional, tag = "2")]
    pub invocation: Option<TextInvocation>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextAttemptOpenSuccess {
    #[prost(uint64, tag = "1")]
    pub attempt_handle: u64,
    #[prost(oneof = "text_attempt_open_success::Output", tags = "2, 3")]
    pub output: Option<text_attempt_open_success::Output>,
}
pub mod text_attempt_open_success {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Output {
        #[prost(message, tag = "2")]
        Request(super::RequestPlan),
        #[prost(message, tag = "3")]
        Response(super::ClientResponse),
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextAttemptOpenResponse {
    #[prost(oneof = "text_attempt_open_response::Result", tags = "1, 2")]
    pub result: Option<text_attempt_open_response::Result>,
}
pub mod text_attempt_open_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::TextAttemptOpenSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}

#[derive(Clone, PartialEq, ::prost::Message)]
pub struct UpstreamResponseHead {
    #[prost(int32, tag = "1")]
    pub status_code: i32,
    #[prost(message, repeated, tag = "2")]
    pub headers: Vec<NameValues>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BufferedUpstreamResponse {
    #[prost(message, optional, tag = "1")]
    pub head: Option<UpstreamResponseHead>,
    #[prost(message, optional, tag = "2")]
    pub body: Option<MessageBody>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextTransformBufferedResponseRequest {
    #[prost(uint64, tag = "1")]
    pub attempt_handle: u64,
    #[prost(message, optional, tag = "2")]
    pub upstream: Option<BufferedUpstreamResponse>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextTransformBufferedResponseSuccess {
    #[prost(message, optional, tag = "1")]
    pub response: Option<ClientResponse>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextTransformBufferedResponseResponse {
    #[prost(
        oneof = "text_transform_buffered_response_response::Result",
        tags = "1, 2"
    )]
    pub result: Option<text_transform_buffered_response_response::Result>,
}
pub mod text_transform_buffered_response_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::TextTransformBufferedResponseSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}

#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextSseOpenRequest {
    #[prost(uint64, tag = "1")]
    pub attempt_handle: u64,
    #[prost(message, optional, tag = "2")]
    pub upstream: Option<UpstreamResponseHead>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextSseOpenSuccess {
    #[prost(int32, tag = "1")]
    pub status_code: i32,
    #[prost(message, repeated, tag = "2")]
    pub headers: Vec<NameValues>,
    #[prost(message, optional, tag = "3")]
    pub outcome: Option<SemanticOutcome>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextSseOpenResponse {
    #[prost(oneof = "text_sse_open_response::Result", tags = "1, 2")]
    pub result: Option<text_sse_open_response::Result>,
}
pub mod text_sse_open_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::TextSseOpenSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}

#[derive(Clone, PartialEq, ::prost::Message)]
pub struct UpstreamSseEvent {
    #[prost(string, tag = "1")]
    pub event_type: String,
    #[prost(bytes = "vec", tag = "2")]
    pub data: Vec<u8>,
    #[prost(string, optional, tag = "3")]
    pub last_event_id: Option<String>,
    #[prost(uint64, optional, tag = "4")]
    pub retry_milliseconds: Option<u64>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextSseTransformEventRequest {
    #[prost(uint64, tag = "1")]
    pub attempt_handle: u64,
    #[prost(message, optional, tag = "2")]
    pub upstream: Option<UpstreamSseEvent>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextSseTransformEventSuccess {
    #[prost(message, repeated, tag = "1")]
    pub events: Vec<ProtocolEventPayload>,
    #[prost(message, optional, tag = "2")]
    pub outcome: Option<SemanticOutcome>,
    #[prost(message, optional, tag = "3")]
    pub usage: Option<UsageReport>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextSseTransformEventResponse {
    #[prost(oneof = "text_sse_transform_event_response::Result", tags = "1, 2")]
    pub result: Option<text_sse_transform_event_response::Result>,
}
pub mod text_sse_transform_event_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::TextSseTransformEventSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}

#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextSseFinishRequest {
    #[prost(uint64, tag = "1")]
    pub attempt_handle: u64,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextSseFinishSuccess {
    #[prost(message, repeated, tag = "1")]
    pub events: Vec<ProtocolEventPayload>,
    #[prost(message, optional, tag = "2")]
    pub outcome: Option<SemanticOutcome>,
    #[prost(message, optional, tag = "3")]
    pub usage: Option<UsageReport>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextSseFinishResponse {
    #[prost(oneof = "text_sse_finish_response::Result", tags = "1, 2")]
    pub result: Option<text_sse_finish_response::Result>,
}
pub mod text_sse_finish_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::TextSseFinishSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}

#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextAttemptCloseRequest {
    #[prost(uint64, tag = "1")]
    pub attempt_handle: u64,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextAttemptCloseSuccess {}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TextAttemptCloseResponse {
    #[prost(oneof = "text_attempt_close_response::Result", tags = "1, 2")]
    pub result: Option<text_attempt_close_response::Result>,
}
pub mod text_attempt_close_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::TextAttemptCloseSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}

#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BlobRef {
    #[prost(uint64, tag = "1")]
    pub id: u64,
    #[prost(int64, tag = "2")]
    pub size: i64,
    #[prost(bytes = "vec", tag = "3")]
    pub sha256: Vec<u8>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MultipartInputPart {
    #[prost(string, tag = "1")]
    pub name: String,
    #[prost(string, optional, tag = "2")]
    pub filename: Option<String>,
    #[prost(message, repeated, tag = "3")]
    pub headers: Vec<NameValues>,
    #[prost(string, tag = "4")]
    pub content_type: String,
    #[prost(oneof = "multipart_input_part::Content", tags = "5, 6")]
    pub content: Option<multipart_input_part::Content>,
}
pub mod multipart_input_part {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Content {
        #[prost(bytes, tag = "5")]
        Inline(Vec<u8>),
        #[prost(message, tag = "6")]
        Blob(super::BlobRef),
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MultipartInput {
    #[prost(message, repeated, tag = "1")]
    pub parts: Vec<MultipartInputPart>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageProtocolRequest {
    #[prost(string, tag = "1")]
    pub protocol_contract: String,
    #[prost(string, tag = "2")]
    pub media_type: String,
    #[prost(oneof = "image_protocol_request::Body", tags = "3, 4")]
    pub body: Option<image_protocol_request::Body>,
}
pub mod image_protocol_request {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Body {
        #[prost(bytes, tag = "3")]
        Json(Vec<u8>),
        #[prost(message, tag = "4")]
        Multipart(super::MultipartInput),
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageInvocation {
    #[prost(message, optional, tag = "1")]
    pub request: Option<ImageProtocolRequest>,
    #[prost(string, tag = "2")]
    pub selected_upstream_model: String,
    #[prost(string, tag = "3")]
    pub response_id: String,
    #[prost(message, repeated, tag = "4")]
    pub blobs: Vec<BlobRef>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BodySegment {
    #[prost(oneof = "body_segment::Source", tags = "1, 2, 3")]
    pub source: Option<body_segment::Source>,
}
pub mod body_segment {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Source {
        #[prost(bytes, tag = "1")]
        Inline(Vec<u8>),
        #[prost(message, tag = "2")]
        Blob(super::BlobRef),
        #[prost(message, tag = "3")]
        Base64(super::BlobRef),
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct CompositeBody {
    #[prost(message, repeated, tag = "1")]
    pub segments: Vec<BodySegment>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MultipartBodyPart {
    #[prost(string, tag = "1")]
    pub name: String,
    #[prost(string, optional, tag = "2")]
    pub filename: Option<String>,
    #[prost(message, repeated, tag = "3")]
    pub headers: Vec<NameValues>,
    #[prost(string, tag = "4")]
    pub content_type: String,
    #[prost(message, optional, tag = "5")]
    pub content: Option<BodySegment>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct MultipartBody {
    #[prost(message, repeated, tag = "1")]
    pub parts: Vec<MultipartBodyPart>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct InlineBody {
    #[prost(bytes = "vec", tag = "1")]
    pub bytes: Vec<u8>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct BodyPlan {
    #[prost(string, tag = "1")]
    pub media_type: String,
    #[prost(oneof = "body_plan::Body", tags = "2, 3, 4, 5, 6")]
    pub body: Option<body_plan::Body>,
}
pub mod body_plan {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Body {
        #[prost(message, tag = "2")]
        Inline(super::InlineBody),
        #[prost(message, tag = "3")]
        Blob(super::BlobRef),
        #[prost(message, tag = "4")]
        Base64(super::BlobRef),
        #[prost(message, tag = "5")]
        Composite(super::CompositeBody),
        #[prost(message, tag = "6")]
        Multipart(super::MultipartBody),
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageAttemptOpenRequest {
    #[prost(bytes = "vec", tag = "1")]
    pub bound_state: Vec<u8>,
    #[prost(message, optional, tag = "2")]
    pub invocation: Option<ImageInvocation>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageAttemptOpenSuccess {
    #[prost(uint64, tag = "1")]
    pub attempt_handle: u64,
    #[prost(oneof = "image_attempt_open_success::Output", tags = "2, 3")]
    pub output: Option<image_attempt_open_success::Output>,
}
pub mod image_attempt_open_success {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Output {
        #[prost(message, tag = "2")]
        Request(super::RequestPlan),
        #[prost(message, tag = "3")]
        Response(super::ImageClientResponse),
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageAttemptOpenResponse {
    #[prost(oneof = "image_attempt_open_response::Result", tags = "1, 2")]
    pub result: Option<image_attempt_open_response::Result>,
}
pub mod image_attempt_open_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::ImageAttemptOpenSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageUpstreamBody {
    #[prost(string, tag = "1")]
    pub media_type: String,
    #[prost(oneof = "image_upstream_body::Content", tags = "2, 3")]
    pub content: Option<image_upstream_body::Content>,
}
pub mod image_upstream_body {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Content {
        #[prost(bytes, tag = "2")]
        Inline(Vec<u8>),
        #[prost(message, tag = "3")]
        Blob(super::BlobRef),
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageUpstreamResponse {
    #[prost(message, optional, tag = "1")]
    pub head: Option<UpstreamResponseHead>,
    #[prost(message, optional, tag = "2")]
    pub body: Option<ImageUpstreamBody>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageClientResponse {
    #[prost(int32, tag = "1")]
    pub status_code: i32,
    #[prost(message, repeated, tag = "2")]
    pub headers: Vec<NameValues>,
    #[prost(string, tag = "3")]
    pub protocol_contract: String,
    #[prost(message, optional, tag = "4")]
    pub body: Option<BodyPlan>,
    #[prost(message, optional, tag = "5")]
    pub outcome: Option<SemanticOutcome>,
    #[prost(message, optional, tag = "6")]
    pub usage: Option<UsageReport>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageTransformBufferedResponseRequest {
    #[prost(uint64, tag = "1")]
    pub attempt_handle: u64,
    #[prost(message, optional, tag = "2")]
    pub upstream: Option<ImageUpstreamResponse>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageTransformBufferedResponseSuccess {
    #[prost(message, optional, tag = "1")]
    pub response: Option<ImageClientResponse>,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageTransformBufferedResponseResponse {
    #[prost(
        oneof = "image_transform_buffered_response_response::Result",
        tags = "1, 2"
    )]
    pub result: Option<image_transform_buffered_response_response::Result>,
}
pub mod image_transform_buffered_response_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::ImageTransformBufferedResponseSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageAttemptCloseRequest {
    #[prost(uint64, tag = "1")]
    pub attempt_handle: u64,
}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageAttemptCloseSuccess {}
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ImageAttemptCloseResponse {
    #[prost(oneof = "image_attempt_close_response::Result", tags = "1, 2")]
    pub result: Option<image_attempt_close_response::Result>,
}
pub mod image_attempt_close_response {
    #[derive(Clone, PartialEq, ::prost::Oneof)]
    pub enum Result {
        #[prost(message, tag = "1")]
        Success(super::ImageAttemptCloseSuccess),
        #[prost(message, tag = "2")]
        Error(super::DriverError),
    }
}
