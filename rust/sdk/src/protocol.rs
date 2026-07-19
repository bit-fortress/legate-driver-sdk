//! Versioned protocol-native JSON packages.

use serde::de::{self, DeserializeOwned, MapAccess, SeqAccess, Visitor};
use serde::{Deserialize, Deserializer, Serialize};
use serde_json::{Map, Number, Value};
use std::fmt;

fn decode<T: DeserializeOwned>(input: &[u8]) -> Result<T, serde_json::Error> {
    let mut deserializer = serde_json::Deserializer::from_slice(input);
    let value = StrictValue::deserialize(&mut deserializer)?;
    deserializer.end()?;
    serde_json::from_value(value.0)
}
fn encode<T: Serialize>(value: &T) -> Result<Vec<u8>, serde_json::Error> {
    serde_json::to_vec(value)
}

struct StrictValue(Value);

impl<'de> Deserialize<'de> for StrictValue {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_any(StrictValueVisitor)
    }
}

struct StrictValueVisitor;

impl<'de> Visitor<'de> for StrictValueVisitor {
    type Value = StrictValue;

    fn expecting(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str("one JSON value without duplicate object keys")
    }

    fn visit_bool<E>(self, value: bool) -> Result<Self::Value, E> {
        Ok(StrictValue(Value::Bool(value)))
    }
    fn visit_i64<E>(self, value: i64) -> Result<Self::Value, E> {
        Ok(StrictValue(Value::Number(Number::from(value))))
    }
    fn visit_u64<E>(self, value: u64) -> Result<Self::Value, E> {
        Ok(StrictValue(Value::Number(Number::from(value))))
    }
    fn visit_f64<E: de::Error>(self, value: f64) -> Result<Self::Value, E> {
        Number::from_f64(value)
            .map(Value::Number)
            .map(StrictValue)
            .ok_or_else(|| E::custom("number is not valid JSON"))
    }
    fn visit_str<E>(self, value: &str) -> Result<Self::Value, E> {
        Ok(StrictValue(Value::String(value.to_owned())))
    }
    fn visit_string<E>(self, value: String) -> Result<Self::Value, E> {
        Ok(StrictValue(Value::String(value)))
    }
    fn visit_none<E>(self) -> Result<Self::Value, E> {
        Ok(StrictValue(Value::Null))
    }
    fn visit_unit<E>(self) -> Result<Self::Value, E> {
        Ok(StrictValue(Value::Null))
    }
    fn visit_seq<A>(self, mut values: A) -> Result<Self::Value, A::Error>
    where
        A: SeqAccess<'de>,
    {
        let mut result = Vec::new();
        while let Some(value) = values.next_element::<StrictValue>()? {
            result.push(value.0);
        }
        Ok(StrictValue(Value::Array(result)))
    }
    fn visit_map<A>(self, mut values: A) -> Result<Self::Value, A::Error>
    where
        A: MapAccess<'de>,
    {
        let mut result = Map::new();
        while let Some(key) = values.next_key::<String>()? {
            if result.contains_key(&key) {
                return Err(de::Error::custom(format!("duplicate object key {key:?}")));
            }
            let value = values.next_value::<StrictValue>()?;
            result.insert(key, value.0);
        }
        Ok(StrictValue(Value::Object(result)))
    }
}

macro_rules! typed_handler {
    () => {
        pub struct OpenAttemptInput {
            pub bound_state: Vec<u8>,
            pub mode: crate::abi::TextMode,
            pub request: Request,
            pub protocol_metadata: ProtocolMetadata,
            pub selected_upstream_model: String,
            pub response_id: String,
        }

        pub enum ResponseBody {
            Success(BufferedSuccessResponse),
            Error(ErrorResponse),
        }

        impl From<BufferedSuccessResponse> for ResponseBody {
            fn from(value: BufferedSuccessResponse) -> Self {
                Self::Success(value)
            }
        }
        impl From<ErrorResponse> for ResponseBody {
            fn from(value: ErrorResponse) -> Self {
                Self::Error(value)
            }
        }

        pub struct ClientResponse {
            pub status_code: i32,
            pub headers: Vec<crate::abi::NameValues>,
            pub body: ResponseBody,
            pub outcome: Option<crate::abi::SemanticOutcome>,
            pub usage: Option<crate::abi::UsageReport>,
        }

        impl ClientResponse {
            /// Builds a typed protocol response. Attach Usage before returning it.
            pub fn new(
                status_code: i32,
                headers: Vec<crate::abi::NameValues>,
                body: impl Into<ResponseBody>,
            ) -> Self {
                Self {
                    status_code,
                    headers,
                    body: body.into(),
                    outcome: None,
                    usage: None,
                }
            }
            pub fn with_outcome(mut self, outcome: crate::abi::SemanticOutcome) -> Self {
                self.outcome = Some(outcome);
                self
            }
            pub fn with_usage(mut self, usage: crate::abi::UsageReport) -> Self {
                self.usage = Some(usage);
                self
            }
        }

        pub struct OpenAttempt<A> {
            pub attempt: A,
            pub output: OpenAttemptOutput,
        }

        pub enum OpenAttemptOutput {
            Request(crate::abi::RequestPlan),
            Response(ClientResponse),
        }

        pub struct StreamEventResult {
            pub events: Vec<StreamEvent>,
            pub outcome: Option<crate::abi::SemanticOutcome>,
            pub usage: Option<crate::abi::UsageReport>,
        }

        impl Default for StreamEventResult {
            fn default() -> Self {
                Self {
                    events: Vec::new(),
                    outcome: None,
                    usage: None,
                }
            }
        }

        pub type StreamFinishResult = StreamEventResult;

        pub trait Handler: Send + Sync + 'static {
            type Attempt: Attempt + Send + 'static;
            fn open_attempt(
                &self,
                input: OpenAttemptInput,
            ) -> crate::DriverResult<OpenAttempt<Self::Attempt>>;
        }

        pub trait Attempt {
            fn transform_buffered(
                &mut self,
                upstream: crate::abi::BufferedUpstreamResponse,
            ) -> crate::DriverResult<ClientResponse>;
            fn open_stream(
                &mut self,
                upstream: crate::abi::UpstreamResponseHead,
            ) -> crate::DriverResult<crate::abi::TextSseOpenSuccess>;
            fn transform_stream_event(
                &mut self,
                upstream: crate::abi::UpstreamSseEvent,
            ) -> crate::DriverResult<StreamEventResult>;
            fn finish_stream(&mut self) -> crate::DriverResult<StreamFinishResult>;
            fn close(self) -> crate::DriverResult<()>;
        }

        pub fn register<H: Handler>(handler: H) -> crate::ProtocolHandlerRegistration {
            crate::ProtocolHandlerRegistration::new(HandlerAdapter(handler))
        }

        struct HandlerAdapter<H>(H);

        impl<H: Handler> crate::ErasedProtocolHandler for HandlerAdapter<H> {
            fn contract(&self) -> &'static str {
                CONTRACT
            }

            fn open(
                &self,
                request: crate::abi::TextAttemptOpenRequest,
            ) -> crate::DriverResult<crate::OpenAttempt<crate::DynamicAttempt>> {
                let invocation = request
                    .invocation
                    .ok_or_else(|| crate::error(crate::ERROR_INVALID_INVOCATION))?;
                let payload = invocation
                    .request
                    .as_ref()
                    .ok_or_else(|| crate::error(crate::ERROR_INVALID_INVOCATION))?;
                if payload.protocol_contract != CONTRACT {
                    return Err(crate::error(crate::ERROR_INVALID_INVOCATION));
                }
                let typed_request = decode_request(&payload.json)
                    .map_err(|_| crate::error(crate::ERROR_INVALID_PROTOCOL_REQUEST))?;
                let metadata = match invocation.protocol_metadata.as_ref() {
                    Some(metadata) if metadata.protocol_contract == CONTRACT => {
                        decode_metadata(&metadata.json)
                    }
                    Some(_) => return Err(crate::error(crate::ERROR_INVALID_INVOCATION)),
                    None => decode_metadata(b"{}"),
                }
                .map_err(|_| crate::error(crate::ERROR_INVALID_PROTOCOL_REQUEST))?;
                let result = self.0.open_attempt(OpenAttemptInput {
                    bound_state: request.bound_state,
                    mode: crate::abi::TextMode::try_from(invocation.mode)
                        .map_err(|_| crate::error(crate::ERROR_UNSUPPORTED_INVOCATION_MODE))?,
                    request: typed_request,
                    protocol_metadata: metadata,
                    selected_upstream_model: invocation.selected_upstream_model,
                    response_id: invocation.response_id,
                })?;
                let OpenAttempt { attempt, output } = result;
                let output = match output {
                    OpenAttemptOutput::Request(request) => {
                        crate::OpenAttemptOutput::Request(request)
                    }
                    OpenAttemptOutput::Response(response) => match raw_response(response) {
                        Ok(response) => crate::OpenAttemptOutput::Response(response),
                        Err(_) => {
                            let _ = attempt.close();
                            return Err(crate::error(crate::ERROR_DRIVER_INTERNAL));
                        }
                    },
                };
                Ok(crate::OpenAttempt {
                    attempt: crate::DynamicAttempt::new(AttemptAdapter(attempt)),
                    output,
                })
            }
        }

        struct AttemptAdapter<A>(A);

        impl<A: Attempt + Send> crate::TextAttempt for AttemptAdapter<A> {
            fn transform_buffered_response(
                &mut self,
                upstream: crate::abi::BufferedUpstreamResponse,
            ) -> crate::DriverResult<crate::abi::TextTransformBufferedResponseSuccess> {
                Ok(crate::abi::TextTransformBufferedResponseSuccess {
                    response: Some(raw_response(self.0.transform_buffered(upstream)?)?),
                })
            }
            fn open_sse(
                &mut self,
                upstream: crate::abi::UpstreamResponseHead,
            ) -> crate::DriverResult<crate::abi::TextSseOpenSuccess> {
                self.0.open_stream(upstream)
            }
            fn transform_sse_event(
                &mut self,
                upstream: crate::abi::UpstreamSseEvent,
            ) -> crate::DriverResult<crate::abi::TextSseTransformEventSuccess> {
                let result = self.0.transform_stream_event(upstream)?;
                let events = raw_events(result.events)?;
                Ok(crate::abi::TextSseTransformEventSuccess {
                    events,
                    outcome: result.outcome,
                    usage: result.usage,
                })
            }
            fn finish_sse(&mut self) -> crate::DriverResult<crate::abi::TextSseFinishSuccess> {
                let result = self.0.finish_stream()?;
                let events = raw_events(result.events)?;
                Ok(crate::abi::TextSseFinishSuccess {
                    events,
                    outcome: result.outcome,
                    usage: result.usage,
                })
            }
            fn close(self) -> crate::DriverResult<()> {
                self.0.close()
            }
        }

        fn raw_response(value: ClientResponse) -> crate::DriverResult<crate::abi::ClientResponse> {
            let json = match value.body {
                ResponseBody::Success(body) => serde_json::to_vec(&body)
                    .map_err(|_| crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE))?,
                ResponseBody::Error(body) => serde_json::to_vec(&body)
                    .map_err(|_| crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE))?,
            };
            Ok(crate::abi::ClientResponse {
                status_code: value.status_code,
                headers: value.headers,
                body: Some(crate::abi::ProtocolPayload {
                    protocol_contract: CONTRACT.to_owned(),
                    media_type: crate::MEDIA_TYPE_JSON.to_owned(),
                    json,
                }),
                outcome: value.outcome,
                usage: value.usage,
            })
        }

        fn raw_events(
            values: Vec<StreamEvent>,
        ) -> crate::DriverResult<Vec<crate::abi::ProtocolEventPayload>> {
            values
                .into_iter()
                .map(|event| {
                    let event_type = event.event_type;
                    if event_type.is_empty() {
                        return Err(crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE));
                    }
                    let json = serde_json::to_vec(&event.body)
                        .map_err(|_| crate::error(crate::ERROR_INVALID_PROTOCOL_RESPONSE))?;
                    Ok(crate::abi::ProtocolEventPayload {
                        protocol_contract: CONTRACT.to_owned(),
                        event_type,
                        json,
                    })
                })
                .collect()
        }
    };
}

pub mod openai {
    pub mod images {
        include!("image_protocol.rs");
    }

    pub mod chatcompletions {
        pub mod v20260718 {
            use serde::{Deserialize, Serialize};
            use serde_json::Value;
            use std::collections::BTreeMap;

            pub const CONTRACT: &str = "openai.chat_completions/2026-07-18";

            #[derive(Clone, Debug, Serialize, Deserialize)]
            pub struct Request {
                pub model: String,
                pub messages: Vec<Value>,
                #[serde(default)]
                pub stream: bool,
                #[serde(flatten)]
                pub extra_fields: BTreeMap<String, Value>,
            }
            #[derive(Clone, Debug, Default, Serialize, Deserialize)]
            pub struct ProtocolMetadata {
                #[serde(flatten)]
                pub extra_fields: BTreeMap<String, Value>,
            }
            #[derive(Clone, Debug, Serialize, Deserialize)]
            #[serde(transparent)]
            pub struct BufferedSuccessResponse(pub Value);
            #[derive(Clone, Debug, Serialize, Deserialize)]
            #[serde(transparent)]
            pub struct ErrorResponse(pub Value);
            #[derive(Clone, Debug)]
            pub struct StreamEvent {
                pub event_type: String,
                pub body: Value,
            }
            #[derive(Clone, Debug)]
            pub struct StreamTermination {
                pub done: bool,
            }

            pub fn decode_request(input: &[u8]) -> Result<Request, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn encode_request(value: &Request) -> Result<Vec<u8>, serde_json::Error> {
                super::super::super::encode(value)
            }
            pub fn decode_metadata(input: &[u8]) -> Result<ProtocolMetadata, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn decode_success(
                input: &[u8],
            ) -> Result<BufferedSuccessResponse, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn decode_error(input: &[u8]) -> Result<ErrorResponse, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn decode_event(
                event_type: String,
                input: &[u8],
            ) -> Result<StreamEvent, serde_json::Error> {
                Ok(StreamEvent {
                    event_type,
                    body: super::super::super::decode(input)?,
                })
            }

            typed_handler!();
        }
    }

    pub mod responses {
        pub mod v20260718 {
            use serde::{Deserialize, Serialize};
            use serde_json::Value;
            use std::collections::BTreeMap;

            pub const CONTRACT: &str = "openai.responses/2026-07-18";
            #[derive(Clone, Debug, Serialize, Deserialize)]
            pub struct Request {
                pub model: String,
                pub input: Value,
                #[serde(default)]
                pub stream: bool,
                #[serde(flatten)]
                pub extra_fields: BTreeMap<String, Value>,
            }
            #[derive(Clone, Debug, Default, Serialize, Deserialize)]
            pub struct ProtocolMetadata {
                #[serde(flatten)]
                pub extra_fields: BTreeMap<String, Value>,
            }
            #[derive(Clone, Debug, Serialize, Deserialize)]
            #[serde(transparent)]
            pub struct BufferedSuccessResponse(pub Value);
            #[derive(Clone, Debug, Serialize, Deserialize)]
            #[serde(transparent)]
            pub struct ErrorResponse(pub Value);
            #[derive(Clone, Debug)]
            pub struct StreamEvent {
                pub event_type: String,
                pub body: Value,
            }
            #[derive(Clone, Debug)]
            pub struct StreamTermination {
                pub event_type: String,
            }
            pub fn decode_request(input: &[u8]) -> Result<Request, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn encode_request(value: &Request) -> Result<Vec<u8>, serde_json::Error> {
                super::super::super::encode(value)
            }
            pub fn decode_metadata(input: &[u8]) -> Result<ProtocolMetadata, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn decode_success(
                input: &[u8],
            ) -> Result<BufferedSuccessResponse, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn decode_error(input: &[u8]) -> Result<ErrorResponse, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn decode_event(
                event_type: String,
                input: &[u8],
            ) -> Result<StreamEvent, serde_json::Error> {
                Ok(StreamEvent {
                    event_type,
                    body: super::super::super::decode(input)?,
                })
            }

            typed_handler!();
        }
    }
}

pub mod anthropic {
    pub mod messages {
        pub mod v20260718 {
            use serde::{Deserialize, Serialize};
            use serde_json::Value;
            use std::collections::BTreeMap;

            pub const CONTRACT: &str = "anthropic.messages/2026-07-18";
            #[derive(Clone, Debug, Serialize, Deserialize)]
            pub struct Request {
                pub model: String,
                pub messages: Vec<Value>,
                #[serde(default)]
                pub stream: bool,
                #[serde(flatten)]
                pub extra_fields: BTreeMap<String, Value>,
            }
            #[derive(Clone, Debug, Default, Serialize, Deserialize)]
            pub struct ProtocolMetadata {
                #[serde(default)]
                pub version: String,
                #[serde(default)]
                pub betas: Vec<String>,
                #[serde(flatten)]
                pub extra_fields: BTreeMap<String, Value>,
            }
            #[derive(Clone, Debug, Serialize, Deserialize)]
            #[serde(transparent)]
            pub struct BufferedSuccessResponse(pub Value);
            #[derive(Clone, Debug, Serialize, Deserialize)]
            #[serde(transparent)]
            pub struct ErrorResponse(pub Value);
            #[derive(Clone, Debug)]
            pub struct StreamEvent {
                pub event_type: String,
                pub body: Value,
            }
            #[derive(Clone, Debug)]
            pub struct StreamTermination {
                pub event_type: String,
            }
            pub fn decode_request(input: &[u8]) -> Result<Request, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn encode_request(value: &Request) -> Result<Vec<u8>, serde_json::Error> {
                super::super::super::encode(value)
            }
            pub fn decode_metadata(input: &[u8]) -> Result<ProtocolMetadata, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn decode_success(
                input: &[u8],
            ) -> Result<BufferedSuccessResponse, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn decode_error(input: &[u8]) -> Result<ErrorResponse, serde_json::Error> {
                super::super::super::decode(input)
            }
            pub fn decode_event(
                event_type: String,
                input: &[u8],
            ) -> Result<StreamEvent, serde_json::Error> {
                Ok(StreamEvent {
                    event_type,
                    body: super::super::super::decode(input)?,
                })
            }

            typed_handler!();
        }
    }
}

#[cfg(test)]
mod tests {
    use super::openai::chatcompletions::v20260718;
    use crate::abi;
    use crate::{Driver, DriverResult, TextBinder};

    #[test]
    fn unknown_fields_round_trip() {
        let mut request = v20260718::decode_request(
            br#"{"model":"group","messages":[],"future_option":{"x":1}}"#,
        )
        .expect("decode");
        request.model = "upstream".to_owned();
        let encoded = v20260718::encode_request(&request).expect("encode");
        let value: serde_json::Value = serde_json::from_slice(&encoded).expect("json");
        assert_eq!(value["future_option"]["x"], 1);
    }

    #[test]
    fn duplicate_keys_are_rejected_at_every_depth() {
        assert!(
            v20260718::decode_request(br#"{"model":"one","model":"two","messages":[]}"#).is_err()
        );
        assert!(v20260718::decode_request(
            br#"{"model":"one","messages":[{"role":"user","role":"assistant"}]}"#
        )
        .is_err());
    }

    struct TestBinder;
    impl TextBinder for TestBinder {
        fn bind_text(&self, _: abi::BindRequest) -> DriverResult<Vec<u8>> {
            Ok(Vec::new())
        }
    }

    struct TestHandler;
    impl v20260718::Handler for TestHandler {
        type Attempt = TestAttempt;
        fn open_attempt(
            &self,
            input: v20260718::OpenAttemptInput,
        ) -> DriverResult<v20260718::OpenAttempt<Self::Attempt>> {
            assert_eq!(input.request.model, "public");
            Ok(v20260718::OpenAttempt {
                attempt: TestAttempt,
                output: v20260718::OpenAttemptOutput::Request(abi::RequestPlan {
                    endpoint_ref: "primary".to_owned(),
                    method: "POST".to_owned(),
                    relative_path: "/v1/chat/completions".to_owned(),
                    body: Some(abi::MessageBody {
                        media_type: crate::MEDIA_TYPE_JSON.to_owned(),
                        payload: b"{}".to_vec(),
                    }),
                    auth: Some(abi::AuthPlan::default()),
                    ..Default::default()
                }),
            })
        }
    }

    struct TestAttempt;
    impl v20260718::Attempt for TestAttempt {
        fn transform_buffered(
            &mut self,
            _: abi::BufferedUpstreamResponse,
        ) -> DriverResult<v20260718::ClientResponse> {
            let body = v20260718::decode_success(b"{}")
                .map_err(|_| crate::error(crate::ERROR_DRIVER_INTERNAL))?;
            Ok(v20260718::ClientResponse::new(200, vec![], body)
                .with_usage(crate::unavailable_usage()))
        }
        fn open_stream(
            &mut self,
            _: abi::UpstreamResponseHead,
        ) -> DriverResult<abi::TextSseOpenSuccess> {
            Ok(abi::TextSseOpenSuccess {
                status_code: 200,
                ..Default::default()
            })
        }
        fn transform_stream_event(
            &mut self,
            _: abi::UpstreamSseEvent,
        ) -> DriverResult<v20260718::StreamEventResult> {
            Ok(v20260718::StreamEventResult::default())
        }
        fn finish_stream(&mut self) -> DriverResult<v20260718::StreamFinishResult> {
            Ok(v20260718::StreamFinishResult::default())
        }
        fn close(self) -> DriverResult<()> {
            Ok(())
        }
    }

    #[test]
    fn dispatcher_rejects_manifest_handler_mismatch() {
        assert!(crate::Dispatcher::new(
            &[crate::PROTOCOL_OPENAI_RESPONSES_2026_07_18],
            TestBinder,
            vec![v20260718::register(TestHandler)],
        )
        .is_err());
        assert!(crate::Dispatcher::new(&[v20260718::CONTRACT], TestBinder, Vec::new(),).is_err());
    }

    #[test]
    fn dispatcher_decodes_and_routes_exact_contract() {
        let dispatcher = crate::Dispatcher::new(
            &[v20260718::CONTRACT],
            TestBinder,
            vec![v20260718::register(TestHandler)],
        )
        .expect("dispatcher");
        let result = dispatcher
            .open_text_attempt(abi::TextAttemptOpenRequest {
                invocation: Some(abi::TextInvocation {
                    mode: abi::TextMode::Buffered as i32,
                    request: Some(abi::ProtocolPayload {
                        protocol_contract: v20260718::CONTRACT.to_owned(),
                        media_type: crate::MEDIA_TYPE_JSON.to_owned(),
                        json: br#"{"model":"public","messages":[]}"#.to_vec(),
                    }),
                    selected_upstream_model: "upstream".to_owned(),
                    response_id: "resp_1".to_owned(),
                    ..Default::default()
                }),
                ..Default::default()
            })
            .expect("open");
        assert!(matches!(
            result.output,
            crate::OpenAttemptOutput::Request(_)
        ));
    }
}
