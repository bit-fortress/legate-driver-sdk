pub mod generations {
    pub mod v20260719 {
        use crate::{abi, DriverResult};
        use serde::{Deserialize, Serialize};
        use serde_json::Value;
        use std::collections::BTreeMap;

        pub const CONTRACT: &str = "openai.images.generations/2026-07-19";

        #[derive(Clone, Debug, Serialize, Deserialize)]
        pub struct Request {
            pub model: String,
            pub prompt: String,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub n: Option<i64>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub quality: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub response_format: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub size: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub style: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub user: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub output_format: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub output_compression: Option<i64>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub background: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub moderation: Option<String>,
            #[serde(flatten)]
            pub extra_fields: BTreeMap<String, Value>,
        }

        #[derive(Clone, Debug, Default)]
        pub struct ProtocolMetadata;

        #[derive(Clone, Debug, Serialize, Deserialize)]
        pub struct SuccessResponse {
            #[serde(skip_serializing_if = "Option::is_none")]
            pub created: Option<i64>,
            #[serde(default, skip_serializing_if = "Vec::is_empty")]
            pub data: Vec<Value>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub usage: Option<Value>,
            #[serde(flatten)]
            pub extra_fields: BTreeMap<String, Value>,
        }

        #[derive(Clone, Debug, Serialize, Deserialize)]
        pub struct ErrorDetail {
            pub message: String,
            #[serde(rename = "type", skip_serializing_if = "Option::is_none")]
            pub error_type: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub param: Option<Value>,
            #[serde(skip_serializing_if = "Option::is_none")]
            pub code: Option<Value>,
            #[serde(flatten)]
            pub extra_fields: BTreeMap<String, Value>,
        }

        #[derive(Clone, Debug, Serialize, Deserialize)]
        pub struct ErrorResponse {
            pub error: ErrorDetail,
            #[serde(flatten)]
            pub extra_fields: BTreeMap<String, Value>,
        }

        pub fn decode_request(input: &[u8]) -> Result<Request, serde_json::Error> {
            let value: Request = crate::protocol::decode(input)?;
            if value.model.is_empty() || value.prompt.is_empty() {
                return Err(json_error("model and prompt are required"));
            }
            Ok(value)
        }
        pub fn encode_request(value: &Request) -> Result<Vec<u8>, serde_json::Error> {
            crate::protocol::encode(value)
        }
        pub fn decode_success(input: &[u8]) -> Result<SuccessResponse, serde_json::Error> {
            crate::protocol::decode(input)
        }
        pub fn decode_error(input: &[u8]) -> Result<ErrorResponse, serde_json::Error> {
            crate::protocol::decode(input)
        }

        pub struct OpenAttemptInput {
            pub bound_state: Vec<u8>,
            pub request: Request,
            pub protocol_metadata: ProtocolMetadata,
            pub selected_upstream_model: String,
            pub response_id: String,
            pub blobs: Vec<abi::BlobRef>,
        }
        pub enum ResponseBody {
            Success(SuccessResponse),
            Error(ErrorResponse),
            CompatibleBlob(abi::BlobRef),
        }
        impl From<SuccessResponse> for ResponseBody { fn from(value: SuccessResponse) -> Self { Self::Success(value) } }
        impl From<ErrorResponse> for ResponseBody { fn from(value: ErrorResponse) -> Self { Self::Error(value) } }
        impl From<abi::BlobRef> for ResponseBody { fn from(value: abi::BlobRef) -> Self { Self::CompatibleBlob(value) } }

        pub struct ClientResponse {
            pub status_code: i32,
            pub headers: Vec<abi::NameValues>,
            pub body: ResponseBody,
            pub outcome: Option<abi::SemanticOutcome>,
            pub usage: Option<abi::UsageReport>,
        }
        impl ClientResponse {
            pub fn new(status_code: i32, headers: Vec<abi::NameValues>, body: impl Into<ResponseBody>) -> Self {
                Self { status_code, headers, body: body.into(), outcome: None, usage: None }
            }
            pub fn with_outcome(mut self, outcome: abi::SemanticOutcome) -> Self { self.outcome = Some(outcome); self }
            pub fn with_usage(mut self, usage: abi::UsageReport) -> Self { self.usage = Some(usage); self }
        }
        pub struct OpenAttempt<A> { pub attempt: A, pub output: OpenAttemptOutput }
        pub enum OpenAttemptOutput { Request(abi::RequestPlan), Response(ClientResponse) }
        pub trait Handler: Send + Sync + 'static {
            type Attempt: Attempt + Send + 'static;
            fn open_attempt(&self, input: OpenAttemptInput) -> DriverResult<OpenAttempt<Self::Attempt>>;
        }
        pub trait Attempt {
            fn transform_buffered(&mut self, upstream: abi::ImageUpstreamResponse) -> DriverResult<ClientResponse>;
            fn close(self) -> DriverResult<()>;
        }

        pub fn register<H: Handler>(handler: H) -> crate::ImageProtocolHandlerRegistration {
            crate::ImageProtocolHandlerRegistration::new(HandlerAdapter(handler))
        }
        struct HandlerAdapter<H>(H);
        impl<H: Handler> crate::image::ErasedImageProtocolHandler for HandlerAdapter<H> {
            fn contract(&self) -> &'static str { CONTRACT }
            fn open(&self, request: abi::ImageAttemptOpenRequest) -> DriverResult<crate::ImageOpenAttempt<crate::DynamicImageAttempt>> {
                let invocation = request.invocation.ok_or_else(|| crate::error(crate::ERROR_INVALID_INVOCATION))?;
                let protocol = invocation.request.ok_or_else(|| crate::error(crate::ERROR_INVALID_INVOCATION))?;
                if protocol.protocol_contract != CONTRACT { return Err(crate::error(crate::ERROR_INVALID_INVOCATION)); }
                let json = match protocol.body {
                    Some(abi::image_protocol_request::Body::Json(value)) => value,
                    _ => return Err(crate::error(crate::ERROR_INVALID_INVOCATION)),
                };
                let typed = match decode_request(&json) {
                    Ok(value) => value,
                    Err(_) => return Ok(invalid_open_response()),
                };
                let result = self.0.open_attempt(OpenAttemptInput {
                    bound_state: request.bound_state, request: typed, protocol_metadata: ProtocolMetadata,
                    selected_upstream_model: invocation.selected_upstream_model,
                    response_id: invocation.response_id, blobs: invocation.blobs,
                })?;
                let OpenAttempt { attempt, output } = result;
                let output = match output {
                    OpenAttemptOutput::Request(value) => crate::ImageOpenAttemptOutput::Request(value),
                    OpenAttemptOutput::Response(value) => match raw_response(value) {
                        Ok(value) => crate::ImageOpenAttemptOutput::Response(value),
                        Err(_) => { let _ = attempt.close(); return Err(crate::error(crate::ERROR_DRIVER_INTERNAL)); }
                    },
                };
                Ok(crate::ImageOpenAttempt { attempt: crate::DynamicImageAttempt::new(AttemptAdapter(attempt)), output })
            }
        }
        struct AttemptAdapter<A>(A);
        impl<A: Attempt + Send> crate::ImageAttempt for AttemptAdapter<A> {
            fn transform_buffered_response(&mut self, upstream: abi::ImageUpstreamResponse) -> DriverResult<abi::ImageTransformBufferedResponseSuccess> {
                Ok(abi::ImageTransformBufferedResponseSuccess { response: Some(raw_response(self.0.transform_buffered(upstream)?)?) })
            }
            fn close(self) -> DriverResult<()> { self.0.close() }
        }
        struct RejectedAttempt;
        impl crate::ImageAttempt for RejectedAttempt {
            fn transform_buffered_response(&mut self, _: abi::ImageUpstreamResponse) -> DriverResult<abi::ImageTransformBufferedResponseSuccess> {
                Err(crate::error(crate::ERROR_INVALID_ATTEMPT_STATE))
            }
            fn close(self) -> DriverResult<()> { Ok(()) }
        }
        fn invalid_open_response() -> crate::ImageOpenAttempt<crate::DynamicImageAttempt> {
            let body = ErrorResponse { error: ErrorDetail {
                message: "Invalid image request.".to_owned(), error_type: Some("invalid_request_error".to_owned()),
                param: None, code: Some(Value::String("invalid_request".to_owned())), extra_fields: BTreeMap::new(),
            }, extra_fields: BTreeMap::new() };
            let response = ClientResponse::new(400, vec![], body)
                .with_outcome(crate::caller_error("invalid_request"))
                .with_usage(crate::unavailable_usage());
            crate::ImageOpenAttempt { attempt: crate::DynamicImageAttempt::new(RejectedAttempt), output: crate::ImageOpenAttemptOutput::Response(raw_response(response).expect("static response")) }
        }
        fn raw_response(value: ClientResponse) -> DriverResult<abi::ImageClientResponse> {
            let body = match value.body {
                ResponseBody::Success(value) => crate::inline_body(crate::MEDIA_TYPE_JSON, crate::protocol::encode(&value).map_err(|_| crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE))?),
                ResponseBody::Error(value) => crate::inline_body(crate::MEDIA_TYPE_JSON, crate::protocol::encode(&value).map_err(|_| crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE))?),
                ResponseBody::CompatibleBlob(value) => crate::blob_body(crate::MEDIA_TYPE_JSON, value),
            };
            if value.usage.is_none() { return Err(crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE)); }
            Ok(abi::ImageClientResponse { status_code: value.status_code, headers: value.headers, protocol_contract: CONTRACT.to_owned(), body: Some(body), outcome: value.outcome, usage: value.usage })
        }
        fn json_error(message: &str) -> serde_json::Error {
            <serde_json::Error as serde::de::Error>::custom(message)
        }
    }
}

pub mod edits {
    pub mod v20260719 {
        use crate::{abi, DriverResult};
        use super::super::generations::v20260719 as generation;

        pub const CONTRACT: &str = "openai.images.edits/2026-07-19";
        pub use generation::{ErrorResponse, SuccessResponse};

        #[derive(Clone, Debug)]
        pub struct Request {
            pub parts: Vec<abi::MultipartInputPart>,
            pub model: String,
            pub prompt: String,
            pub image_parts: Vec<usize>,
            pub mask_part: Option<usize>,
        }
        #[derive(Clone, Debug, Default)]
        pub struct ProtocolMetadata;

        pub fn decode_request(input: abi::MultipartInput) -> Result<Request, &'static str> {
            if input.parts.is_empty() { return Err("multipart request is required"); }
            let mut result = Request { parts: input.parts, model: String::new(), prompt: String::new(), image_parts: vec![], mask_part: None };
            for (index, part) in result.parts.iter().enumerate() {
                match part.name.as_str() {
                    "model" | "prompt" => {
                        let Some(abi::multipart_input_part::Content::Inline(value)) = part.content.as_ref() else { return Err("model and prompt must be text fields"); };
                        let text = std::str::from_utf8(value).map_err(|_| "text field is not UTF-8")?;
                        if part.name == "model" && result.model.is_empty() { result.model = text.to_owned(); }
                        if part.name == "prompt" && result.prompt.is_empty() { result.prompt = text.to_owned(); }
                    }
                    "image" => match part.content.as_ref() {
                        Some(abi::multipart_input_part::Content::Blob(_)) if part.filename.is_some() => result.image_parts.push(index),
                        _ => return Err("image must use a file part"),
                    },
                    "mask" => match part.content.as_ref() {
                        Some(abi::multipart_input_part::Content::Blob(_)) if part.filename.is_some() && result.mask_part.is_none() => result.mask_part = Some(index),
                        _ => return Err("mask must use one file part"),
                    },
                    _ => {}
                }
            }
            if result.model.trim().is_empty() || result.prompt.trim().is_empty() || result.image_parts.is_empty() { return Err("model, prompt, and image are required"); }
            Ok(result)
        }

        impl Request {
            pub fn multipart_body(&self, selected_model: &str) -> Result<abi::BodyPlan, &'static str> {
                if selected_model.is_empty() { return Err("selected upstream model is required"); }
                let mut saw_model = false;
                let mut parts = Vec::with_capacity(self.parts.len() + 1);
                for part in &self.parts {
                    let content = if part.name == "model" && part.filename.is_none() {
                        saw_model = true;
                        crate::inline_segment(selected_model.as_bytes().to_vec())
                    } else {
                        match part.content.as_ref() {
                            Some(abi::multipart_input_part::Content::Inline(value)) => crate::inline_segment(value.clone()),
                            Some(abi::multipart_input_part::Content::Blob(value)) => crate::blob_segment(value.clone()),
                            None => return Err("multipart part content is required"),
                        }
                    };
                    parts.push(abi::MultipartBodyPart { name: part.name.clone(), filename: part.filename.clone(), headers: public_part_headers(&part.headers), content_type: part.content_type.clone(), content: Some(content) });
                }
                if !saw_model {
                    parts.push(abi::MultipartBodyPart { name: "model".to_owned(), content: Some(crate::inline_segment(selected_model.as_bytes().to_vec())), ..Default::default() });
                }
                Ok(crate::multipart_body("multipart/form-data", parts))
            }
        }

        pub struct OpenAttemptInput { pub bound_state: Vec<u8>, pub request: Request, pub protocol_metadata: ProtocolMetadata, pub selected_upstream_model: String, pub response_id: String, pub blobs: Vec<abi::BlobRef> }
        pub enum ResponseBody { Success(SuccessResponse), Error(ErrorResponse), CompatibleBlob(abi::BlobRef) }
        impl From<SuccessResponse> for ResponseBody { fn from(value: SuccessResponse) -> Self { Self::Success(value) } }
        impl From<ErrorResponse> for ResponseBody { fn from(value: ErrorResponse) -> Self { Self::Error(value) } }
        impl From<abi::BlobRef> for ResponseBody { fn from(value: abi::BlobRef) -> Self { Self::CompatibleBlob(value) } }
        pub struct ClientResponse { pub status_code: i32, pub headers: Vec<abi::NameValues>, pub body: ResponseBody, pub outcome: Option<abi::SemanticOutcome>, pub usage: Option<abi::UsageReport> }
        impl ClientResponse {
            pub fn new(status_code: i32, headers: Vec<abi::NameValues>, body: impl Into<ResponseBody>) -> Self { Self { status_code, headers, body: body.into(), outcome: None, usage: None } }
            pub fn with_outcome(mut self, outcome: abi::SemanticOutcome) -> Self { self.outcome = Some(outcome); self }
            pub fn with_usage(mut self, usage: abi::UsageReport) -> Self { self.usage = Some(usage); self }
        }
        pub struct OpenAttempt<A> { pub attempt: A, pub output: OpenAttemptOutput }
        pub enum OpenAttemptOutput { Request(abi::RequestPlan), Response(ClientResponse) }
        pub trait Handler: Send + Sync + 'static { type Attempt: Attempt + Send + 'static; fn open_attempt(&self, input: OpenAttemptInput) -> DriverResult<OpenAttempt<Self::Attempt>>; }
        pub trait Attempt { fn transform_buffered(&mut self, upstream: abi::ImageUpstreamResponse) -> DriverResult<ClientResponse>; fn close(self) -> DriverResult<()>; }
        pub fn register<H: Handler>(handler: H) -> crate::ImageProtocolHandlerRegistration { crate::ImageProtocolHandlerRegistration::new(HandlerAdapter(handler)) }
        struct HandlerAdapter<H>(H);
        impl<H: Handler> crate::image::ErasedImageProtocolHandler for HandlerAdapter<H> {
            fn contract(&self) -> &'static str { CONTRACT }
            fn open(&self, request: abi::ImageAttemptOpenRequest) -> DriverResult<crate::ImageOpenAttempt<crate::DynamicImageAttempt>> {
                let invocation = request.invocation.ok_or_else(|| crate::error(crate::ERROR_INVALID_INVOCATION))?;
                let protocol = invocation.request.ok_or_else(|| crate::error(crate::ERROR_INVALID_INVOCATION))?;
                if protocol.protocol_contract != CONTRACT { return Err(crate::error(crate::ERROR_INVALID_INVOCATION)); }
                let multipart = match protocol.body { Some(abi::image_protocol_request::Body::Multipart(value)) => value, _ => return Err(crate::error(crate::ERROR_INVALID_INVOCATION)) };
                let typed = match decode_request(multipart) { Ok(value) => value, Err(_) => return Ok(invalid_open_response()) };
                let result = self.0.open_attempt(OpenAttemptInput { bound_state: request.bound_state, request: typed, protocol_metadata: ProtocolMetadata, selected_upstream_model: invocation.selected_upstream_model, response_id: invocation.response_id, blobs: invocation.blobs })?;
                let OpenAttempt { attempt, output } = result;
                let output = match output { OpenAttemptOutput::Request(value) => crate::ImageOpenAttemptOutput::Request(value), OpenAttemptOutput::Response(value) => match raw_response(value) { Ok(value) => crate::ImageOpenAttemptOutput::Response(value), Err(_) => { let _ = attempt.close(); return Err(crate::error(crate::ERROR_DRIVER_INTERNAL)); } } };
                Ok(crate::ImageOpenAttempt { attempt: crate::DynamicImageAttempt::new(AttemptAdapter(attempt)), output })
            }
        }
        struct AttemptAdapter<A>(A);
        impl<A: Attempt + Send> crate::ImageAttempt for AttemptAdapter<A> {
            fn transform_buffered_response(&mut self, upstream: abi::ImageUpstreamResponse) -> DriverResult<abi::ImageTransformBufferedResponseSuccess> { Ok(abi::ImageTransformBufferedResponseSuccess { response: Some(raw_response(self.0.transform_buffered(upstream)?)?) }) }
            fn close(self) -> DriverResult<()> { self.0.close() }
        }
        struct RejectedAttempt;
        impl crate::ImageAttempt for RejectedAttempt { fn transform_buffered_response(&mut self, _: abi::ImageUpstreamResponse) -> DriverResult<abi::ImageTransformBufferedResponseSuccess> { Err(crate::error(crate::ERROR_INVALID_ATTEMPT_STATE)) } fn close(self) -> DriverResult<()> { Ok(()) } }
        fn invalid_open_response() -> crate::ImageOpenAttempt<crate::DynamicImageAttempt> {
            let body = generation::ErrorResponse { error: generation::ErrorDetail { message: "Invalid image request.".to_owned(), error_type: Some("invalid_request_error".to_owned()), param: None, code: Some(serde_json::Value::String("invalid_request".to_owned())), extra_fields: Default::default() }, extra_fields: Default::default() };
            let response = ClientResponse::new(400, vec![], body).with_outcome(crate::caller_error("invalid_request")).with_usage(crate::unavailable_usage());
            crate::ImageOpenAttempt { attempt: crate::DynamicImageAttempt::new(RejectedAttempt), output: crate::ImageOpenAttemptOutput::Response(raw_response(response).expect("static response")) }
        }
        fn raw_response(value: ClientResponse) -> DriverResult<abi::ImageClientResponse> {
            let body = match value.body { ResponseBody::Success(value) => crate::inline_body(crate::MEDIA_TYPE_JSON, crate::protocol::encode(&value).map_err(|_| crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE))?), ResponseBody::Error(value) => crate::inline_body(crate::MEDIA_TYPE_JSON, crate::protocol::encode(&value).map_err(|_| crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE))?), ResponseBody::CompatibleBlob(value) => crate::blob_body(crate::MEDIA_TYPE_JSON, value) };
            if value.usage.is_none() { return Err(crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE)); }
            Ok(abi::ImageClientResponse { status_code: value.status_code, headers: value.headers, protocol_contract: CONTRACT.to_owned(), body: Some(body), outcome: value.outcome, usage: value.usage })
        }
        fn public_part_headers(input: &[abi::NameValues]) -> Vec<abi::NameValues> { input.iter().filter(|header| !header.name.eq_ignore_ascii_case("content-disposition") && !header.name.eq_ignore_ascii_case("content-type") && !header.name.eq_ignore_ascii_case("content-length")).cloned().collect() }
    }
}
