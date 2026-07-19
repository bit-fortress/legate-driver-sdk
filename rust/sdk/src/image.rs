use crate::{
    abi, error, internal_error, valid_outcome, valid_request_plan, valid_response_hook_error,
    valid_usage, DriverResult,
};
use prost::Message;
use std::collections::BTreeMap;
use std::sync::Mutex;

pub trait ImageBinder: Sync {
    fn bind_image(&self, request: abi::BindRequest) -> DriverResult<Vec<u8>>;
}

pub trait ImageDriver: Sync {
    type Attempt: ImageAttempt + Send;
    fn bind(&self, request: abi::BindRequest) -> DriverResult<abi::BindSuccess>;
    fn open_image_attempt(
        &self,
        request: abi::ImageAttemptOpenRequest,
    ) -> DriverResult<ImageOpenAttempt<Self::Attempt>>;
}

pub trait ImageAttempt {
    fn transform_buffered_response(
        &mut self,
        upstream: abi::ImageUpstreamResponse,
    ) -> DriverResult<abi::ImageTransformBufferedResponseSuccess>;
    fn close(self) -> DriverResult<()>;
}

pub struct ImageOpenAttempt<A> {
    pub attempt: A,
    pub output: ImageOpenAttemptOutput,
}

pub enum ImageOpenAttemptOutput {
    Request(abi::RequestPlan),
    Response(abi::ImageClientResponse),
}

trait ErasedImageAttempt: Send {
    fn transform(
        &mut self,
        upstream: abi::ImageUpstreamResponse,
    ) -> DriverResult<abi::ImageTransformBufferedResponseSuccess>;
    fn close(self: Box<Self>) -> DriverResult<()>;
}

impl<T: ImageAttempt + Send> ErasedImageAttempt for T {
    fn transform(
        &mut self,
        upstream: abi::ImageUpstreamResponse,
    ) -> DriverResult<abi::ImageTransformBufferedResponseSuccess> {
        ImageAttempt::transform_buffered_response(self, upstream)
    }
    fn close(self: Box<Self>) -> DriverResult<()> {
        ImageAttempt::close(*self)
    }
}

pub struct DynamicImageAttempt(Box<dyn ErasedImageAttempt>);

impl DynamicImageAttempt {
    pub(crate) fn new<T: ImageAttempt + Send + 'static>(attempt: T) -> Self {
        Self(Box::new(attempt))
    }
}

impl ImageAttempt for DynamicImageAttempt {
    fn transform_buffered_response(
        &mut self,
        upstream: abi::ImageUpstreamResponse,
    ) -> DriverResult<abi::ImageTransformBufferedResponseSuccess> {
        self.0.transform(upstream)
    }
    fn close(self) -> DriverResult<()> {
        self.0.close()
    }
}

pub(crate) trait ErasedImageProtocolHandler: Send + Sync {
    fn contract(&self) -> &'static str;
    fn open(
        &self,
        request: abi::ImageAttemptOpenRequest,
    ) -> DriverResult<ImageOpenAttempt<DynamicImageAttempt>>;
}

pub struct ImageProtocolHandlerRegistration(pub(crate) Box<dyn ErasedImageProtocolHandler>);

impl ImageProtocolHandlerRegistration {
    pub(crate) fn new(handler: impl ErasedImageProtocolHandler + 'static) -> Self {
        Self(Box::new(handler))
    }
}

pub struct ImageDispatcher<B> {
    binder: B,
    contracts: Vec<String>,
    handlers: BTreeMap<String, Box<dyn ErasedImageProtocolHandler>>,
}

impl<B: ImageBinder> ImageDispatcher<B> {
    pub fn new(
        declared_contracts: &[&str],
        binder: B,
        registrations: Vec<ImageProtocolHandlerRegistration>,
    ) -> Result<Self, &'static str> {
        if declared_contracts.is_empty() {
            return Err("manifest image protocol contracts are required");
        }
        let mut contracts = declared_contracts
            .iter()
            .map(|value| (*value).to_owned())
            .collect::<Vec<_>>();
        contracts.sort();
        if contracts.iter().enumerate().any(|(index, contract)| {
            !valid_image_contract(contract) || index > 0 && contract == &contracts[index - 1]
        }) {
            return Err("manifest contains an invalid or duplicate image protocol contract");
        }
        let mut handlers = BTreeMap::new();
        for registration in registrations {
            let contract = registration.0.contract();
            if !valid_image_contract(contract)
                || handlers
                    .insert(contract.to_owned(), registration.0)
                    .is_some()
            {
                return Err("image protocol handler registration is invalid or duplicate");
            }
        }
        if handlers.len() != contracts.len()
            || contracts
                .iter()
                .any(|contract| !handlers.contains_key(contract))
        {
            return Err("manifest declarations and image handler registrations differ");
        }
        Ok(Self {
            binder,
            contracts,
            handlers,
        })
    }
}

impl<B: ImageBinder> ImageDriver for ImageDispatcher<B> {
    type Attempt = DynamicImageAttempt;

    fn bind(&self, request: abi::BindRequest) -> DriverResult<abi::BindSuccess> {
        Ok(abi::BindSuccess {
            bound_state: self.binder.bind_image(request)?,
            capabilities: Some(abi::bind_success::Capabilities::ImageCapabilities(
                abi::ImageCapabilities {
                    protocol_contracts: self.contracts.clone(),
                },
            )),
        })
    }

    fn open_image_attempt(
        &self,
        request: abi::ImageAttemptOpenRequest,
    ) -> DriverResult<ImageOpenAttempt<Self::Attempt>> {
        let contract = request
            .invocation
            .as_ref()
            .and_then(|invocation| invocation.request.as_ref())
            .map(|request| request.protocol_contract.as_str())
            .ok_or_else(|| error(crate::ERROR_INVALID_INVOCATION))?;
        self.handlers
            .get(contract)
            .ok_or_else(|| error(crate::ERROR_INVALID_INVOCATION))?
            .open(request)
    }
}

struct ImageGuestState<A> {
    attempt: Option<A>,
    handle: u64,
    next_handle: u64,
    contract: String,
    terminal: bool,
}

pub struct ImageGuest<D: ImageDriver> {
    driver: D,
    state: Mutex<ImageGuestState<D::Attempt>>,
}

impl<D: ImageDriver> ImageGuest<D> {
    pub fn new(driver: D) -> Self {
        Self {
            driver,
            state: Mutex::new(ImageGuestState {
                attempt: None,
                handle: 0,
                next_handle: 1,
                contract: String::new(),
                terminal: false,
            }),
        }
    }

    pub fn handle_bind(&self, input: &[u8]) -> Vec<u8> {
        let result = match abi::BindRequest::decode(input) {
            Ok(request) => match self.driver.bind(request) {
                Ok(success)
                    if success.capabilities.as_ref().is_some_and(
                        |capabilities| match capabilities {
                            abi::bind_success::Capabilities::ImageCapabilities(value) => {
                                valid_image_capabilities(value)
                            }
                            abi::bind_success::Capabilities::TextCapabilities(_) => false,
                        },
                    ) =>
                {
                    abi::bind_response::Result::Success(success)
                }
                Ok(_) => abi::bind_response::Result::Error(internal_error()),
                Err(error) => abi::bind_response::Result::Error(*error),
            },
            Err(_) => abi::bind_response::Result::Error(internal_error()),
        };
        abi::BindResponse {
            result: Some(result),
        }
        .encode_to_vec()
    }

    pub fn handle_image_attempt_open(&self, input: &[u8]) -> Vec<u8> {
        let request = match abi::ImageAttemptOpenRequest::decode(input) {
            Ok(request) => request,
            Err(_) => return image_open_error(internal_error()),
        };
        let Some(invocation) = request.invocation.as_ref() else {
            return image_open_error(*error(crate::ERROR_INVALID_INVOCATION));
        };
        if !valid_image_invocation(invocation) {
            return image_open_error(*error(crate::ERROR_INVALID_INVOCATION));
        }
        let contract = invocation
            .request
            .as_ref()
            .expect("validated request")
            .protocol_contract
            .clone();
        let mut state = match self.state.lock() {
            Ok(state) => state,
            Err(_) => return image_open_error(internal_error()),
        };
        if state.attempt.is_some() {
            return image_open_error(*error(crate::ERROR_INVALID_ATTEMPT_STATE));
        }
        let opened = match self.driver.open_image_attempt(request) {
            Ok(opened) => opened,
            Err(error) => return image_open_error(*error),
        };
        let ImageOpenAttempt { attempt, output } = opened;
        let output = match output {
            ImageOpenAttemptOutput::Request(request) if valid_request_plan(&request) => {
                abi::image_attempt_open_success::Output::Request(request)
            }
            ImageOpenAttemptOutput::Response(response)
                if valid_image_response(&response, &contract) =>
            {
                abi::image_attempt_open_success::Output::Response(response)
            }
            _ => {
                let _ = attempt.close();
                return image_open_error(internal_error());
            }
        };
        let handle = state.next_handle;
        state.next_handle = state.next_handle.checked_add(1).unwrap_or(1);
        state.handle = handle;
        state.contract = contract;
        state.terminal = false;
        state.attempt = Some(attempt);
        abi::ImageAttemptOpenResponse {
            result: Some(abi::image_attempt_open_response::Result::Success(
                abi::ImageAttemptOpenSuccess {
                    attempt_handle: handle,
                    output: Some(output),
                },
            )),
        }
        .encode_to_vec()
    }

    pub fn handle_image_transform_buffered_response(&self, input: &[u8]) -> Vec<u8> {
        let request = match abi::ImageTransformBufferedResponseRequest::decode(input) {
            Ok(request) => request,
            Err(_) => return image_transform_error(internal_error()),
        };
        let Some(upstream) = request.upstream else {
            return image_transform_error(internal_error());
        };
        if !valid_image_upstream(&upstream) {
            return image_transform_error(internal_error());
        }
        let mut state = match self.state.lock() {
            Ok(state) => state,
            Err(_) => return image_transform_error(internal_error()),
        };
        if request.attempt_handle == 0
            || request.attempt_handle != state.handle
            || state.attempt.is_none()
        {
            return image_transform_error(*error(crate::ERROR_INVALID_ATTEMPT));
        }
        if state.terminal {
            return image_transform_error(*error(crate::ERROR_INVALID_ATTEMPT_STATE));
        }
        state.terminal = true;
        let result = state
            .attempt
            .as_mut()
            .expect("validated attempt")
            .transform_buffered_response(upstream);
        match result {
            Ok(success)
                if success
                    .response
                    .as_ref()
                    .is_some_and(|response| valid_image_response(response, &state.contract)) =>
            {
                abi::ImageTransformBufferedResponseResponse {
                    result: Some(
                        abi::image_transform_buffered_response_response::Result::Success(success),
                    ),
                }
                .encode_to_vec()
            }
            Ok(_) => image_transform_error(internal_error()),
            Err(error) if valid_response_hook_error(&error) => image_transform_error(*error),
            Err(_) => image_transform_error(internal_error()),
        }
    }

    pub fn handle_image_attempt_close(&self, input: &[u8]) -> Vec<u8> {
        let request = match abi::ImageAttemptCloseRequest::decode(input) {
            Ok(request) => request,
            Err(_) => return image_close_error(internal_error()),
        };
        let attempt = {
            let mut state = match self.state.lock() {
                Ok(state) => state,
                Err(_) => return image_close_error(internal_error()),
            };
            if request.attempt_handle == 0
                || request.attempt_handle != state.handle
                || state.attempt.is_none()
            {
                return image_close_error(*error(crate::ERROR_INVALID_ATTEMPT));
            }
            state.handle = 0;
            state.contract.clear();
            state.terminal = false;
            state.attempt.take().expect("validated attempt")
        };
        match attempt.close() {
            Ok(()) => abi::ImageAttemptCloseResponse {
                result: Some(abi::image_attempt_close_response::Result::Success(
                    abi::ImageAttemptCloseSuccess {},
                )),
            }
            .encode_to_vec(),
            Err(error) => image_close_error(*error),
        }
    }
}

pub fn inline_body(media_type: impl Into<String>, bytes: Vec<u8>) -> abi::BodyPlan {
    abi::BodyPlan {
        media_type: media_type.into(),
        body: Some(abi::body_plan::Body::Inline(abi::InlineBody { bytes })),
    }
}
pub fn blob_body(media_type: impl Into<String>, blob: abi::BlobRef) -> abi::BodyPlan {
    abi::BodyPlan {
        media_type: media_type.into(),
        body: Some(abi::body_plan::Body::Blob(blob)),
    }
}
pub fn base64_body(media_type: impl Into<String>, blob: abi::BlobRef) -> abi::BodyPlan {
    abi::BodyPlan {
        media_type: media_type.into(),
        body: Some(abi::body_plan::Body::Base64(blob)),
    }
}
pub fn composite_body(
    media_type: impl Into<String>,
    segments: Vec<abi::BodySegment>,
) -> abi::BodyPlan {
    abi::BodyPlan {
        media_type: media_type.into(),
        body: Some(abi::body_plan::Body::Composite(abi::CompositeBody {
            segments,
        })),
    }
}
pub fn multipart_body(
    media_type: impl Into<String>,
    parts: Vec<abi::MultipartBodyPart>,
) -> abi::BodyPlan {
    abi::BodyPlan {
        media_type: media_type.into(),
        body: Some(abi::body_plan::Body::Multipart(abi::MultipartBody {
            parts,
        })),
    }
}
pub fn inline_segment(bytes: Vec<u8>) -> abi::BodySegment {
    abi::BodySegment {
        source: Some(abi::body_segment::Source::Inline(bytes)),
    }
}
pub fn blob_segment(blob: abi::BlobRef) -> abi::BodySegment {
    abi::BodySegment {
        source: Some(abi::body_segment::Source::Blob(blob)),
    }
}
pub fn base64_segment(blob: abi::BlobRef) -> abi::BodySegment {
    abi::BodySegment {
        source: Some(abi::body_segment::Source::Base64(blob)),
    }
}

fn valid_image_contract(value: &str) -> bool {
    matches!(
        value,
        crate::PROTOCOL_OPENAI_IMAGE_GENERATIONS_2026_07_19
            | crate::PROTOCOL_OPENAI_IMAGE_EDITS_2026_07_19
    )
}
fn valid_image_capabilities(value: &abi::ImageCapabilities) -> bool {
    !value.protocol_contracts.is_empty()
        && value
            .protocol_contracts
            .windows(2)
            .all(|pair| pair[0] < pair[1])
        && value
            .protocol_contracts
            .iter()
            .all(|contract| valid_image_contract(contract))
}
fn valid_blob(value: &abi::BlobRef) -> bool {
    value.id != 0 && value.size >= 0 && value.sha256.len() == 32
}
fn valid_image_invocation(value: &abi::ImageInvocation) -> bool {
    let Some(request) = value.request.as_ref() else {
        return false;
    };
    if !valid_image_contract(&request.protocol_contract)
        || value.selected_upstream_model.is_empty()
        || value.response_id.is_empty()
    {
        return false;
    }
    match (&*request.protocol_contract, request.body.as_ref()) {
        (
            crate::PROTOCOL_OPENAI_IMAGE_GENERATIONS_2026_07_19,
            Some(abi::image_protocol_request::Body::Json(value)),
        ) => !value.is_empty(),
        (
            crate::PROTOCOL_OPENAI_IMAGE_EDITS_2026_07_19,
            Some(abi::image_protocol_request::Body::Multipart(value)),
        ) => !value.parts.is_empty(),
        _ => false,
    }
}
fn valid_image_upstream(value: &abi::ImageUpstreamResponse) -> bool {
    value.head.is_some()
        && value.body.as_ref().is_some_and(|body| {
            !body.media_type.is_empty()
                && body.content.as_ref().is_some_and(|content| match content {
                    abi::image_upstream_body::Content::Inline(_) => true,
                    abi::image_upstream_body::Content::Blob(value) => valid_blob(value),
                })
        })
}
fn valid_body_plan(value: &abi::BodyPlan) -> bool {
    !value.media_type.is_empty()
        && value.body.as_ref().is_some_and(|body| match body {
            abi::body_plan::Body::Inline(_) => true,
            abi::body_plan::Body::Blob(value) | abi::body_plan::Body::Base64(value) => {
                valid_blob(value)
            }
            abi::body_plan::Body::Composite(value) => {
                !value.segments.is_empty() && value.segments.iter().all(valid_segment)
            }
            abi::body_plan::Body::Multipart(value) => {
                !value.parts.is_empty()
                    && value.parts.iter().all(|part| {
                        !part.name.is_empty() && part.content.as_ref().is_some_and(valid_segment)
                    })
            }
        })
}
fn valid_segment(value: &abi::BodySegment) -> bool {
    value.source.as_ref().is_some_and(|source| match source {
        abi::body_segment::Source::Inline(_) => true,
        abi::body_segment::Source::Blob(value) | abi::body_segment::Source::Base64(value) => {
            valid_blob(value)
        }
    })
}
fn valid_image_response(value: &abi::ImageClientResponse, contract: &str) -> bool {
    (200..=599).contains(&value.status_code)
        && value.protocol_contract == contract
        && value.body.as_ref().is_some_and(valid_body_plan)
        && value.outcome.as_ref().is_none_or(valid_outcome)
        && value.usage.as_ref().is_some_and(valid_usage)
}
fn image_open_error(value: abi::DriverError) -> Vec<u8> {
    abi::ImageAttemptOpenResponse {
        result: Some(abi::image_attempt_open_response::Result::Error(value)),
    }
    .encode_to_vec()
}
fn image_transform_error(value: abi::DriverError) -> Vec<u8> {
    abi::ImageTransformBufferedResponseResponse {
        result: Some(abi::image_transform_buffered_response_response::Result::Error(value)),
    }
    .encode_to_vec()
}
fn image_close_error(value: abi::DriverError) -> Vec<u8> {
    abi::ImageAttemptCloseResponse {
        result: Some(abi::image_attempt_close_response::Result::Error(value)),
    }
    .encode_to_vec()
}
