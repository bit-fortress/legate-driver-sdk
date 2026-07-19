//! Legate protocol-native Driver ABI v1 guest SDK.

use prost::Message;
use std::collections::{BTreeMap, HashMap};
use std::sync::{LazyLock, Mutex};

pub mod abi;
mod image;
pub mod protocol;
pub use image::*;

pub const PROTOCOL_OPENAI_CHAT_COMPLETIONS_2026_07_18: &str = "openai.chat_completions/2026-07-18";
pub const PROTOCOL_OPENAI_RESPONSES_2026_07_18: &str = "openai.responses/2026-07-18";
pub const PROTOCOL_ANTHROPIC_MESSAGES_2026_07_18: &str = "anthropic.messages/2026-07-18";
pub const PROTOCOL_OPENAI_IMAGE_GENERATIONS_2026_07_19: &str =
    "openai.images.generations/2026-07-19";
pub const PROTOCOL_OPENAI_IMAGE_EDITS_2026_07_19: &str = "openai.images.edits/2026-07-19";
pub const MEDIA_TYPE_JSON: &str = "application/json";

pub const ERROR_INVALID_CONFIG: &str = "invalid_config";
pub const ERROR_UNSUPPORTED_FEATURE: &str = "unsupported_feature";
pub const ERROR_UNSUPPORTED_INVOCATION_MODE: &str = "unsupported_invocation_mode";
pub const ERROR_INVALID_PROTOCOL_REQUEST: &str = "protocol_request_invalid";
pub const ERROR_INVALID_PROTOCOL_RESPONSE: &str = "protocol_response_invalid";
pub const ERROR_INVALID_INVOCATION: &str = "invalid_invocation";
pub const ERROR_MALFORMED_UPSTREAM_RESPONSE: &str = "malformed_upstream_response";
pub const ERROR_UPSTREAM_STREAM_TRUNCATED: &str = "upstream_stream_truncated";
pub const ERROR_INVALID_ATTEMPT: &str = "invalid_attempt";
pub const ERROR_INVALID_ATTEMPT_STATE: &str = "invalid_attempt_state";
pub const ERROR_DRIVER_INTERNAL: &str = "driver_internal";
pub const ERROR_DRIVER_RESOURCE_EXHAUSTED: &str = "driver_resource_exhausted";

pub type DriverResult<T> = Result<T, Box<abi::DriverError>>;

static ALLOCATIONS: LazyLock<Mutex<HashMap<u32, Box<[u8]>>>> =
    LazyLock::new(|| Mutex::new(HashMap::new()));

pub trait Driver: Sync {
    type Attempt: TextAttempt + Send;
    fn bind(&self, request: abi::BindRequest) -> DriverResult<abi::BindSuccess>;
    fn open_text_attempt(
        &self,
        request: abi::TextAttemptOpenRequest,
    ) -> DriverResult<OpenAttempt<Self::Attempt>>;
}

pub struct OpenAttempt<A> {
    pub attempt: A,
    pub output: OpenAttemptOutput,
}

pub enum OpenAttemptOutput {
    Request(abi::RequestPlan),
    Response(abi::ClientResponse),
}

pub trait TextAttempt {
    fn transform_buffered_response(
        &mut self,
        upstream: abi::BufferedUpstreamResponse,
    ) -> DriverResult<abi::TextTransformBufferedResponseSuccess>;
    fn open_sse(
        &mut self,
        upstream: abi::UpstreamResponseHead,
    ) -> DriverResult<abi::TextSseOpenSuccess>;
    fn transform_sse_event(
        &mut self,
        upstream: abi::UpstreamSseEvent,
    ) -> DriverResult<abi::TextSseTransformEventSuccess>;
    fn finish_sse(&mut self) -> DriverResult<abi::TextSseFinishSuccess>;
    fn close(self) -> DriverResult<()>;
}

trait ErasedAttempt: Send {
    fn erased_transform_buffered_response(
        &mut self,
        upstream: abi::BufferedUpstreamResponse,
    ) -> DriverResult<abi::TextTransformBufferedResponseSuccess>;
    fn erased_open_sse(
        &mut self,
        upstream: abi::UpstreamResponseHead,
    ) -> DriverResult<abi::TextSseOpenSuccess>;
    fn erased_transform_sse_event(
        &mut self,
        upstream: abi::UpstreamSseEvent,
    ) -> DriverResult<abi::TextSseTransformEventSuccess>;
    fn erased_finish_sse(&mut self) -> DriverResult<abi::TextSseFinishSuccess>;
    fn erased_close(self: Box<Self>) -> DriverResult<()>;
}

impl<T: TextAttempt + Send> ErasedAttempt for T {
    fn erased_transform_buffered_response(
        &mut self,
        upstream: abi::BufferedUpstreamResponse,
    ) -> DriverResult<abi::TextTransformBufferedResponseSuccess> {
        TextAttempt::transform_buffered_response(self, upstream)
    }
    fn erased_open_sse(
        &mut self,
        upstream: abi::UpstreamResponseHead,
    ) -> DriverResult<abi::TextSseOpenSuccess> {
        TextAttempt::open_sse(self, upstream)
    }
    fn erased_transform_sse_event(
        &mut self,
        upstream: abi::UpstreamSseEvent,
    ) -> DriverResult<abi::TextSseTransformEventSuccess> {
        TextAttempt::transform_sse_event(self, upstream)
    }
    fn erased_finish_sse(&mut self) -> DriverResult<abi::TextSseFinishSuccess> {
        TextAttempt::finish_sse(self)
    }
    fn erased_close(self: Box<Self>) -> DriverResult<()> {
        TextAttempt::close(*self)
    }
}

/// Type-erased attempt used only inside the protocol dispatcher.
pub struct DynamicAttempt(Box<dyn ErasedAttempt>);

impl DynamicAttempt {
    pub(crate) fn new<T: TextAttempt + Send + 'static>(attempt: T) -> Self {
        Self(Box::new(attempt))
    }
}

impl TextAttempt for DynamicAttempt {
    fn transform_buffered_response(
        &mut self,
        upstream: abi::BufferedUpstreamResponse,
    ) -> DriverResult<abi::TextTransformBufferedResponseSuccess> {
        self.0.erased_transform_buffered_response(upstream)
    }
    fn open_sse(
        &mut self,
        upstream: abi::UpstreamResponseHead,
    ) -> DriverResult<abi::TextSseOpenSuccess> {
        self.0.erased_open_sse(upstream)
    }
    fn transform_sse_event(
        &mut self,
        upstream: abi::UpstreamSseEvent,
    ) -> DriverResult<abi::TextSseTransformEventSuccess> {
        self.0.erased_transform_sse_event(upstream)
    }
    fn finish_sse(&mut self) -> DriverResult<abi::TextSseFinishSuccess> {
        self.0.erased_finish_sse()
    }
    fn close(self) -> DriverResult<()> {
        self.0.erased_close()
    }
}

pub trait TextBinder: Sync {
    fn bind_text(&self, request: abi::BindRequest) -> DriverResult<Vec<u8>>;
}

trait ErasedProtocolHandler: Send + Sync {
    fn contract(&self) -> &'static str;
    fn open(
        &self,
        request: abi::TextAttemptOpenRequest,
    ) -> DriverResult<OpenAttempt<DynamicAttempt>>;
}

pub struct ProtocolHandlerRegistration(Box<dyn ErasedProtocolHandler>);

impl ProtocolHandlerRegistration {
    pub(crate) fn new(handler: impl ErasedProtocolHandler + 'static) -> Self {
        Self(Box::new(handler))
    }
}

pub struct Dispatcher<B> {
    binder: B,
    contracts: Vec<String>,
    handlers: BTreeMap<String, Box<dyn ErasedProtocolHandler>>,
}

impl<B: TextBinder> Dispatcher<B> {
    pub fn new(
        declared_contracts: &[&str],
        binder: B,
        registrations: Vec<ProtocolHandlerRegistration>,
    ) -> Result<Self, &'static str> {
        if declared_contracts.is_empty() {
            return Err("manifest protocol contracts are required");
        }
        let mut contracts = declared_contracts
            .iter()
            .map(|value| (*value).to_owned())
            .collect::<Vec<_>>();
        contracts.sort();
        if contracts.iter().enumerate().any(|(index, contract)| {
            !valid_contract(contract) || index > 0 && contract == &contracts[index - 1]
        }) {
            return Err("manifest contains an invalid or duplicate protocol contract");
        }
        let mut handlers = BTreeMap::new();
        for registration in registrations {
            let contract = registration.0.contract();
            if !valid_contract(contract)
                || handlers
                    .insert(contract.to_owned(), registration.0)
                    .is_some()
            {
                return Err("protocol handler registration is invalid or duplicate");
            }
        }
        if handlers.len() != contracts.len()
            || contracts
                .iter()
                .any(|contract| !handlers.contains_key(contract))
        {
            return Err("manifest declarations and handler registrations differ");
        }
        Ok(Self {
            binder,
            contracts,
            handlers,
        })
    }

    pub fn protocol_contracts(&self) -> &[String] {
        &self.contracts
    }
}

impl<B: TextBinder> Driver for Dispatcher<B> {
    type Attempt = DynamicAttempt;

    fn bind(&self, request: abi::BindRequest) -> DriverResult<abi::BindSuccess> {
        Ok(abi::BindSuccess {
            bound_state: self.binder.bind_text(request)?,
            capabilities: Some(abi::bind_success::Capabilities::TextCapabilities(
                abi::TextCapabilities {
                    protocol_contracts: self.contracts.clone(),
                },
            )),
        })
    }

    fn open_text_attempt(
        &self,
        request: abi::TextAttemptOpenRequest,
    ) -> DriverResult<OpenAttempt<Self::Attempt>> {
        let contract = request
            .invocation
            .as_ref()
            .and_then(|invocation| invocation.request.as_ref())
            .map(|payload| payload.protocol_contract.as_str())
            .ok_or_else(|| error(ERROR_INVALID_INVOCATION))?;
        self.handlers
            .get(contract)
            .ok_or_else(|| error(ERROR_INVALID_INVOCATION))?
            .open(request)
    }
}

struct GuestState<A> {
    attempt: Option<A>,
    handle: u64,
    next_handle: u64,
    mode: i32,
    phase: AttemptPhase,
    saw_outcome: bool,
    saw_usage: bool,
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum AttemptPhase {
    None,
    Ready,
    SseOpen,
    Terminal,
}

pub struct Guest<D: Driver> {
    driver: D,
    state: Mutex<GuestState<D::Attempt>>,
}

impl<D: Driver> Guest<D> {
    pub const fn new(driver: D) -> Self {
        Self {
            driver,
            state: Mutex::new(GuestState {
                attempt: None,
                handle: 0,
                next_handle: 1,
                mode: abi::TextMode::Unspecified as i32,
                phase: AttemptPhase::None,
                saw_outcome: false,
                saw_usage: false,
            }),
        }
    }

    pub fn handle_bind(&self, input: &[u8]) -> Vec<u8> {
        let result = match abi::BindRequest::decode(input) {
            Ok(request) => match self.driver.bind(request) {
                Ok(success)
                    if success.capabilities.as_ref().is_some_and(
                        |capabilities| match capabilities {
                            abi::bind_success::Capabilities::TextCapabilities(value) => {
                                valid_capabilities(value)
                            }
                            abi::bind_success::Capabilities::ImageCapabilities(_) => false,
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

    pub fn handle_text_attempt_open(&self, input: &[u8]) -> Vec<u8> {
        let request = match abi::TextAttemptOpenRequest::decode(input) {
            Ok(request) => request,
            Err(_) => return open_error(internal_error()),
        };
        let Some(invocation) = request.invocation.as_ref() else {
            return open_error(*error(ERROR_INVALID_INVOCATION));
        };
        if !valid_invocation(invocation) {
            return open_error(*error(ERROR_INVALID_INVOCATION));
        }
        if invocation.mode != abi::TextMode::Buffered as i32
            && invocation.mode != abi::TextMode::Sse as i32
        {
            return open_error(*error(ERROR_UNSUPPORTED_INVOCATION_MODE));
        }
        let mut state = match self.state.lock() {
            Ok(state) => state,
            Err(_) => return open_error(internal_error()),
        };
        if state.attempt.is_some() {
            return open_error(*error(ERROR_INVALID_ATTEMPT_STATE));
        }
        let mode = invocation.mode;
        let opened = match self.driver.open_text_attempt(request) {
            Ok(opened) => opened,
            Err(error) => return open_error(*error),
        };
        let OpenAttempt { attempt, output } = opened;
        let output = match output {
            OpenAttemptOutput::Request(request) if valid_request_plan(&request) => {
                abi::text_attempt_open_success::Output::Request(request)
            }
            OpenAttemptOutput::Response(response) if valid_client_response(&response, true) => {
                abi::text_attempt_open_success::Output::Response(response)
            }
            _ => {
                let _ = attempt.close();
                return open_error(internal_error());
            }
        };
        let handle = state.next_handle;
        state.next_handle = state.next_handle.checked_add(1).unwrap_or(1);
        state.handle = handle;
        state.attempt = Some(attempt);
        state.mode = mode;
        state.phase = AttemptPhase::Ready;
        state.saw_outcome = false;
        state.saw_usage = false;
        abi::TextAttemptOpenResponse {
            result: Some(abi::text_attempt_open_response::Result::Success(
                abi::TextAttemptOpenSuccess {
                    attempt_handle: handle,
                    output: Some(output),
                },
            )),
        }
        .encode_to_vec()
    }

    pub fn handle_text_transform_buffered_response(&self, input: &[u8]) -> Vec<u8> {
        let request = match abi::TextTransformBufferedResponseRequest::decode(input) {
            Ok(request) => request,
            Err(_) => return buffered_error(internal_error()),
        };
        let Some(upstream) = request.upstream else {
            return buffered_error(internal_error());
        };
        let mut state = match self.state.lock() {
            Ok(state) => state,
            Err(_) => return buffered_error(internal_error()),
        };
        if let Err(error) = validate_phase(
            &state,
            request.attempt_handle,
            AttemptPhase::Ready,
            abi::TextMode::Buffered as i32,
        ) {
            return buffered_error(*error);
        }
        state.phase = AttemptPhase::Terminal;
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
                    .is_some_and(|value| valid_client_response(value, true)) =>
            {
                abi::TextTransformBufferedResponseResponse {
                    result: Some(
                        abi::text_transform_buffered_response_response::Result::Success(success),
                    ),
                }
                .encode_to_vec()
            }
            Ok(_) => buffered_error(internal_error()),
            Err(error) if valid_response_hook_error(&error) => buffered_error(*error),
            Err(_) => buffered_error(internal_error()),
        }
    }

    pub fn handle_text_sse_open(&self, input: &[u8]) -> Vec<u8> {
        let request = match abi::TextSseOpenRequest::decode(input) {
            Ok(request) => request,
            Err(_) => return sse_open_error(internal_error()),
        };
        let Some(upstream) = request.upstream else {
            return sse_open_error(internal_error());
        };
        let mut state = match self.state.lock() {
            Ok(state) => state,
            Err(_) => return sse_open_error(internal_error()),
        };
        if let Err(error) = validate_phase(
            &state,
            request.attempt_handle,
            AttemptPhase::Ready,
            abi::TextMode::Sse as i32,
        ) {
            return sse_open_error(*error);
        }
        state.phase = AttemptPhase::Terminal;
        let result = state
            .attempt
            .as_mut()
            .expect("validated attempt")
            .open_sse(upstream);
        match result {
            Ok(success) if valid_sse_open(&success) => {
                state.phase = AttemptPhase::SseOpen;
                state.saw_outcome = success.outcome.is_some();
                abi::TextSseOpenResponse {
                    result: Some(abi::text_sse_open_response::Result::Success(success)),
                }
                .encode_to_vec()
            }
            Ok(_) => sse_open_error(internal_error()),
            Err(error) if valid_response_hook_error(&error) => sse_open_error(*error),
            Err(_) => sse_open_error(internal_error()),
        }
    }

    pub fn handle_text_sse_transform_event(&self, input: &[u8]) -> Vec<u8> {
        let request = match abi::TextSseTransformEventRequest::decode(input) {
            Ok(request) => request,
            Err(_) => return sse_event_error(internal_error()),
        };
        let Some(upstream) = request.upstream else {
            return sse_event_error(internal_error());
        };
        let mut state = match self.state.lock() {
            Ok(state) => state,
            Err(_) => return sse_event_error(internal_error()),
        };
        if let Err(error) = validate_phase(
            &state,
            request.attempt_handle,
            AttemptPhase::SseOpen,
            abi::TextMode::Sse as i32,
        ) {
            return sse_event_error(*error);
        }
        let result = state
            .attempt
            .as_mut()
            .expect("validated attempt")
            .transform_sse_event(upstream);
        match result {
            Ok(success)
                if valid_stream_result(
                    &success.events,
                    success.outcome.as_ref(),
                    success.usage.as_ref(),
                ) =>
            {
                state.saw_outcome |= success.outcome.is_some();
                state.saw_usage |= success.usage.is_some();
                abi::TextSseTransformEventResponse {
                    result: Some(abi::text_sse_transform_event_response::Result::Success(
                        success,
                    )),
                }
                .encode_to_vec()
            }
            Ok(_) => {
                state.phase = AttemptPhase::Terminal;
                sse_event_error(internal_error())
            }
            Err(error) if valid_response_hook_error(&error) => {
                state.phase = AttemptPhase::Terminal;
                sse_event_error(*error)
            }
            Err(_) => {
                state.phase = AttemptPhase::Terminal;
                sse_event_error(internal_error())
            }
        }
    }

    pub fn handle_text_sse_finish(&self, input: &[u8]) -> Vec<u8> {
        let request = match abi::TextSseFinishRequest::decode(input) {
            Ok(request) => request,
            Err(_) => return sse_finish_error(internal_error()),
        };
        let mut state = match self.state.lock() {
            Ok(state) => state,
            Err(_) => return sse_finish_error(internal_error()),
        };
        if let Err(error) = validate_phase(
            &state,
            request.attempt_handle,
            AttemptPhase::SseOpen,
            abi::TextMode::Sse as i32,
        ) {
            return sse_finish_error(*error);
        }
        state.phase = AttemptPhase::Terminal;
        let result = state
            .attempt
            .as_mut()
            .expect("validated attempt")
            .finish_sse();
        match result {
            Ok(success)
                if valid_stream_result(
                    &success.events,
                    success.outcome.as_ref(),
                    success.usage.as_ref(),
                ) && (state.saw_outcome || success.outcome.is_some())
                    && (state.saw_usage || success.usage.is_some()) =>
            {
                abi::TextSseFinishResponse {
                    result: Some(abi::text_sse_finish_response::Result::Success(success)),
                }
                .encode_to_vec()
            }
            Ok(_) => sse_finish_error(internal_error()),
            Err(error) if valid_response_hook_error(&error) => sse_finish_error(*error),
            Err(_) => sse_finish_error(internal_error()),
        }
    }

    pub fn handle_text_attempt_close(&self, input: &[u8]) -> Vec<u8> {
        let request = match abi::TextAttemptCloseRequest::decode(input) {
            Ok(request) => request,
            Err(_) => return close_error(internal_error()),
        };
        let attempt = {
            let mut state = match self.state.lock() {
                Ok(state) => state,
                Err(_) => return close_error(internal_error()),
            };
            if !valid_handle(&state, request.attempt_handle) {
                return close_error(*error(ERROR_INVALID_ATTEMPT));
            }
            state.handle = 0;
            state.mode = abi::TextMode::Unspecified as i32;
            state.phase = AttemptPhase::None;
            state.saw_outcome = false;
            state.saw_usage = false;
            state.attempt.take().expect("validated attempt")
        };
        match attempt.close() {
            Ok(()) => abi::TextAttemptCloseResponse {
                result: Some(abi::text_attempt_close_response::Result::Success(
                    abi::TextAttemptCloseSuccess {},
                )),
            }
            .encode_to_vec(),
            Err(error) => close_error(*error),
        }
    }
}

fn valid_handle<A>(state: &GuestState<A>, handle: u64) -> bool {
    handle != 0 && handle == state.handle && state.attempt.is_some()
}
fn validate_phase<A>(
    state: &GuestState<A>,
    handle: u64,
    phase: AttemptPhase,
    mode: i32,
) -> DriverResult<()> {
    if !valid_handle(state, handle) {
        return Err(error(ERROR_INVALID_ATTEMPT));
    }
    if state.phase != phase || state.mode != mode {
        return Err(error(ERROR_INVALID_ATTEMPT_STATE));
    }
    Ok(())
}
fn valid_contract(value: &str) -> bool {
    matches!(
        value,
        PROTOCOL_OPENAI_CHAT_COMPLETIONS_2026_07_18
            | PROTOCOL_OPENAI_RESPONSES_2026_07_18
            | PROTOCOL_ANTHROPIC_MESSAGES_2026_07_18
    )
}
fn valid_payload(value: &abi::ProtocolPayload) -> bool {
    valid_contract(&value.protocol_contract)
        && value.media_type == MEDIA_TYPE_JSON
        && !value.json.is_empty()
}
fn valid_invocation(value: &abi::TextInvocation) -> bool {
    value.request.as_ref().is_some_and(valid_payload)
        && value.protocol_metadata.as_ref().is_none_or(valid_payload)
        && !value.selected_upstream_model.is_empty()
        && !value.response_id.is_empty()
}
fn valid_capabilities(value: &abi::TextCapabilities) -> bool {
    !value.protocol_contracts.is_empty()
        && value
            .protocol_contracts
            .windows(2)
            .all(|pair| pair[0] < pair[1])
        && value
            .protocol_contracts
            .iter()
            .all(|contract| valid_contract(contract))
}
fn valid_request_plan(value: &abi::RequestPlan) -> bool {
    !value.endpoint_ref.is_empty()
        && !value.method.is_empty()
        && !value.relative_path.is_empty()
        && (value.body.is_some() != value.body_plan.is_some())
        && value.auth.is_some()
}
fn valid_client_response(value: &abi::ClientResponse, require_usage: bool) -> bool {
    (200..=599).contains(&value.status_code)
        && value.body.as_ref().is_some_and(valid_payload)
        && value.outcome.as_ref().is_none_or(valid_outcome)
        && (!require_usage || value.usage.is_some())
        && value.usage.as_ref().is_none_or(valid_usage)
}
fn valid_sse_open(value: &abi::TextSseOpenSuccess) -> bool {
    (200..=599).contains(&value.status_code) && value.outcome.as_ref().is_none_or(valid_outcome)
}
fn valid_stream_result(
    events: &[abi::ProtocolEventPayload],
    outcome: Option<&abi::SemanticOutcome>,
    usage: Option<&abi::UsageReport>,
) -> bool {
    events.iter().all(|event| {
        valid_contract(&event.protocol_contract)
            && !event.event_type.is_empty()
            && !event.json.is_empty()
    }) && outcome.is_none_or(valid_outcome)
        && usage.is_none_or(valid_usage)
}
fn valid_outcome(value: &abi::SemanticOutcome) -> bool {
    if value.vendor_code.len() > 256 || value.vendor_code.chars().any(char::is_control) {
        return false;
    }
    match abi::SemanticOutcomeClass::try_from(value.class) {
        Ok(abi::SemanticOutcomeClass::Success) => value.vendor_code.is_empty(),
        Ok(
            abi::SemanticOutcomeClass::CallerError
            | abi::SemanticOutcomeClass::EndpointError
            | abi::SemanticOutcomeClass::MappingError,
        ) => true,
        _ => false,
    }
}
fn valid_usage(value: &abi::UsageReport) -> bool {
    if !matches!(
        abi::UsageProvenance::try_from(value.provenance),
        Ok(abi::UsageProvenance::UpstreamReported | abi::UsageProvenance::DriverAccumulated)
    ) {
        return false;
    }
    let counts = [
        value.input_tokens,
        value.output_tokens,
        value.cached_tokens,
        value.reasoning_tokens,
    ];
    if counts.iter().flatten().any(|count| *count < 0) {
        return false;
    }
    let known = counts.iter().filter(|count| count.is_some()).count();
    match abi::UsageStatus::try_from(value.status) {
        Ok(abi::UsageStatus::Final)
            if value.input_tokens.is_some() && value.output_tokens.is_some() => {}
        Ok(abi::UsageStatus::Partial) if known > 0 => {}
        Ok(abi::UsageStatus::Unavailable) if known == 0 => {}
        _ => return false,
    }
    if value
        .input_tokens
        .zip(value.cached_tokens)
        .is_some_and(|(input, cached)| cached > input)
        || value
            .output_tokens
            .zip(value.reasoning_tokens)
            .is_some_and(|(output, reasoning)| reasoning > output)
    {
        return false;
    }
    value
        .input_tokens
        .zip(value.output_tokens)
        .is_none_or(|(input, output)| input.checked_add(output).is_some())
}
fn valid_response_hook_error(value: &abi::DriverError) -> bool {
    value.usage.as_ref().is_none_or(|usage| {
        valid_usage(usage)
            && matches!(
                abi::UsageStatus::try_from(usage.status),
                Ok(abi::UsageStatus::Partial | abi::UsageStatus::Unavailable)
            )
    })
}

/// Builds a protocol-native response. Attach Usage before returning it.
pub fn response(
    status_code: i32,
    headers: Vec<abi::NameValues>,
    body: abi::ProtocolPayload,
) -> abi::ClientResponse {
    abi::ClientResponse {
        status_code,
        headers,
        body: Some(body),
        outcome: None,
        usage: None,
    }
}
pub fn success() -> abi::SemanticOutcome {
    abi::SemanticOutcome {
        class: abi::SemanticOutcomeClass::Success as i32,
        vendor_code: String::new(),
    }
}
pub fn caller_error(code: impl Into<String>) -> abi::SemanticOutcome {
    abi::SemanticOutcome {
        class: abi::SemanticOutcomeClass::CallerError as i32,
        vendor_code: code.into(),
    }
}
pub fn endpoint_error(code: impl Into<String>) -> abi::SemanticOutcome {
    abi::SemanticOutcome {
        class: abi::SemanticOutcomeClass::EndpointError as i32,
        vendor_code: code.into(),
    }
}
pub fn mapping_error(code: impl Into<String>) -> abi::SemanticOutcome {
    abi::SemanticOutcome {
        class: abi::SemanticOutcomeClass::MappingError as i32,
        vendor_code: code.into(),
    }
}
pub fn unavailable_usage() -> abi::UsageReport {
    abi::UsageReport {
        status: abi::UsageStatus::Unavailable as i32,
        provenance: abi::UsageProvenance::UpstreamReported as i32,
        ..Default::default()
    }
}
pub fn error(code: &str) -> Box<abi::DriverError> {
    Box::new(abi::DriverError {
        code: code.to_owned(),
        ..Default::default()
    })
}
fn internal_error() -> abi::DriverError {
    *error(ERROR_DRIVER_INTERNAL)
}

macro_rules! error_encoder {
    ($fn_name:ident, $response:ident, $module:ident) => {
        fn $fn_name(error: abi::DriverError) -> Vec<u8> {
            abi::$response {
                result: Some(abi::$module::Result::Error(error)),
            }
            .encode_to_vec()
        }
    };
}
error_encoder!(
    open_error,
    TextAttemptOpenResponse,
    text_attempt_open_response
);
error_encoder!(
    buffered_error,
    TextTransformBufferedResponseResponse,
    text_transform_buffered_response_response
);
error_encoder!(sse_open_error, TextSseOpenResponse, text_sse_open_response);
error_encoder!(
    sse_event_error,
    TextSseTransformEventResponse,
    text_sse_transform_event_response
);
error_encoder!(
    sse_finish_error,
    TextSseFinishResponse,
    text_sse_finish_response
);
error_encoder!(
    close_error,
    TextAttemptCloseResponse,
    text_attempt_close_response
);

pub fn is_empty_json_object(input: &[u8]) -> bool {
    let trimmed = input.strip_prefix(&[]).unwrap_or(input);
    let start = trimmed
        .iter()
        .position(|value| !matches!(value, b' ' | b'\t' | b'\n' | b'\r'));
    let end = trimmed
        .iter()
        .rposition(|value| !matches!(value, b' ' | b'\t' | b'\n' | b'\r'));
    matches!((start, end), (Some(start), Some(end)) if trimmed[start] == b'{' && trimmed[end] == b'}' && trimmed[start + 1..end].iter().all(|value| matches!(value, b' ' | b'\t' | b'\n' | b'\r')))
}

pub extern "C" fn legate_sdk_alloc(size: u32) -> u32 {
    if size == 0 {
        return 0;
    }
    store_buffer(vec![0_u8; size as usize].into_boxed_slice())
}
/// Releases one exact live SDK allocation.
///
/// # Safety
/// `pointer` and `size` must describe a buffer returned by this SDK.
pub unsafe extern "C" fn legate_sdk_free(pointer: u32, size: u32) {
    if pointer == 0 || size == 0 {
        return;
    }
    if let Ok(mut allocations) = ALLOCATIONS.lock() {
        if allocations
            .get(&pointer)
            .is_some_and(|buffer| buffer.len() == size as usize)
        {
            allocations.remove(&pointer);
        }
    }
}
/// Invokes a guest hook over one exact live SDK allocation.
///
/// # Safety
/// `pointer` and `size` must describe a buffer returned by this SDK, or both
/// must be zero.
pub unsafe fn invoke(pointer: u32, size: u32, handler: impl FnOnce(&[u8]) -> Vec<u8>) -> u64 {
    let input = if pointer == 0 && size == 0 {
        Vec::new()
    } else {
        let Ok(allocations) = ALLOCATIONS.lock() else {
            return 0;
        };
        let Some(buffer) = allocations.get(&pointer) else {
            return 0;
        };
        if buffer.len() != size as usize {
            return 0;
        }
        buffer.to_vec()
    };
    let output = handler(&input).into_boxed_slice();
    if output.is_empty() || output.len() > u32::MAX as usize {
        return 0;
    }
    let length = output.len() as u32;
    let output_pointer = store_buffer(output);
    if output_pointer == 0 {
        0
    } else {
        ((output_pointer as u64) << 32) | length as u64
    }
}
fn store_buffer(mut buffer: Box<[u8]>) -> u32 {
    let pointer = buffer.as_mut_ptr() as usize as u32;
    if pointer == 0 {
        return 0;
    }
    let Ok(mut allocations) = ALLOCATIONS.lock() else {
        return 0;
    };
    if allocations.contains_key(&pointer) {
        return 0;
    }
    allocations.insert(pointer, buffer);
    pointer
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicBool, Ordering};
    use std::sync::Arc;

    struct MissingUsageDriver {
        closed: Arc<AtomicBool>,
    }

    struct CloseTrackingAttempt {
        closed: Arc<AtomicBool>,
    }

    impl Driver for MissingUsageDriver {
        type Attempt = CloseTrackingAttempt;

        fn bind(&self, _: abi::BindRequest) -> DriverResult<abi::BindSuccess> {
            unreachable!()
        }

        fn open_text_attempt(
            &self,
            _: abi::TextAttemptOpenRequest,
        ) -> DriverResult<OpenAttempt<Self::Attempt>> {
            Ok(OpenAttempt {
                attempt: CloseTrackingAttempt {
                    closed: Arc::clone(&self.closed),
                },
                output: OpenAttemptOutput::Response(response(
                    400,
                    vec![],
                    abi::ProtocolPayload {
                        protocol_contract: PROTOCOL_OPENAI_CHAT_COMPLETIONS_2026_07_18.to_owned(),
                        media_type: MEDIA_TYPE_JSON.to_owned(),
                        json: br#"{"error":{"message":"rejected"}}"#.to_vec(),
                    },
                )),
            })
        }
    }

    impl TextAttempt for CloseTrackingAttempt {
        fn transform_buffered_response(
            &mut self,
            _: abi::BufferedUpstreamResponse,
        ) -> DriverResult<abi::TextTransformBufferedResponseSuccess> {
            Err(error(ERROR_DRIVER_INTERNAL))
        }

        fn open_sse(
            &mut self,
            _: abi::UpstreamResponseHead,
        ) -> DriverResult<abi::TextSseOpenSuccess> {
            Err(error(ERROR_DRIVER_INTERNAL))
        }

        fn transform_sse_event(
            &mut self,
            _: abi::UpstreamSseEvent,
        ) -> DriverResult<abi::TextSseTransformEventSuccess> {
            Err(error(ERROR_DRIVER_INTERNAL))
        }

        fn finish_sse(&mut self) -> DriverResult<abi::TextSseFinishSuccess> {
            Err(error(ERROR_DRIVER_INTERNAL))
        }

        fn close(self) -> DriverResult<()> {
            self.closed.store(true, Ordering::SeqCst);
            Ok(())
        }
    }

    #[test]
    fn exact_contract_capabilities_are_sorted() {
        assert!(valid_capabilities(&abi::TextCapabilities {
            protocol_contracts: vec![PROTOCOL_OPENAI_CHAT_COMPLETIONS_2026_07_18.to_owned()]
        }));
        assert!(!valid_capabilities(&abi::TextCapabilities {
            protocol_contracts: vec![
                PROTOCOL_OPENAI_RESPONSES_2026_07_18.to_owned(),
                PROTOCOL_OPENAI_CHAT_COMPLETIONS_2026_07_18.to_owned()
            ]
        }));
    }

    #[test]
    fn outcome_contains_no_retry_instruction() {
        assert!(valid_outcome(&endpoint_error("overloaded")));
        assert!(valid_outcome(&mapping_error("model_not_found")));
    }

    #[test]
    fn immediate_response_requires_usage_and_closes_attempt_when_invalid() {
        let closed = Arc::new(AtomicBool::new(false));
        let guest = Guest::new(MissingUsageDriver {
            closed: Arc::clone(&closed),
        });
        let request = abi::TextAttemptOpenRequest {
            bound_state: vec![],
            invocation: Some(abi::TextInvocation {
                mode: abi::TextMode::Buffered as i32,
                request: Some(abi::ProtocolPayload {
                    protocol_contract: PROTOCOL_OPENAI_CHAT_COMPLETIONS_2026_07_18.to_owned(),
                    media_type: MEDIA_TYPE_JSON.to_owned(),
                    json: br#"{"model":"group","messages":[]}"#.to_vec(),
                }),
                selected_upstream_model: "upstream".to_owned(),
                response_id: "resp_1".to_owned(),
                ..Default::default()
            }),
        };
        let output = guest.handle_text_attempt_open(&request.encode_to_vec());
        let response = abi::TextAttemptOpenResponse::decode(output.as_slice()).expect("decode");
        assert!(matches!(
            response.result,
            Some(abi::text_attempt_open_response::Result::Error(ref error))
                if error.code == ERROR_DRIVER_INTERNAL
        ));
        assert!(closed.load(Ordering::SeqCst));
    }
}
