use legate_driver_sdk as sdk;
use sdk::abi;
use sdk::protocol::openai::chatcompletions::v20260718 as chat;
use serde_json::Value;
use std::sync::LazyLock;

struct OpenAiChatBinder;
struct OpenAiChatHandler;
struct OpenAiChatAttempt {
    mode: abi::TextMode,
    finished: bool,
    closed: bool,
}

impl sdk::TextBinder for OpenAiChatBinder {
    fn bind_text(&self, request: abi::BindRequest) -> sdk::DriverResult<Vec<u8>> {
        if !sdk::is_empty_json_object(&request.config_json)
            || !request.endpoint_refs.iter().any(|value| value == "primary")
            || !request
                .credential_slots
                .iter()
                .any(|slot| slot.name == "api_key" && slot.configured)
        {
            return Err(sdk::error(sdk::ERROR_INVALID_CONFIG));
        }
        Ok(b"openai-chat-compatible-v1".to_vec())
    }
}

impl chat::Handler for OpenAiChatHandler {
    type Attempt = OpenAiChatAttempt;

    fn open_attempt(
        &self,
        mut input: chat::OpenAttemptInput,
    ) -> sdk::DriverResult<chat::OpenAttempt<Self::Attempt>> {
        if input.bound_state != b"openai-chat-compatible-v1" {
            return Err(sdk::error(sdk::ERROR_INVALID_INVOCATION));
        }
        input.request.model = input.selected_upstream_model;
        let streaming = input.mode == abi::TextMode::Sse;
        input.request.stream = streaming;
        if streaming {
            input.request.extra_fields.insert(
                "stream_options".to_owned(),
                serde_json::json!({"include_usage": true}),
            );
        }
        let encoded = chat::encode_request(&input.request)
            .map_err(|_| sdk::error(sdk::ERROR_DRIVER_INTERNAL))?;
        Ok(chat::OpenAttempt {
            attempt: OpenAiChatAttempt {
                mode: input.mode,
                finished: false,
                closed: false,
            },
            output: chat::OpenAttemptOutput::Request(abi::RequestPlan {
                endpoint_ref: "primary".to_owned(),
                method: "POST".to_owned(),
                relative_path: "/v1/chat/completions".to_owned(),
                body: Some(abi::MessageBody {
                    media_type: sdk::MEDIA_TYPE_JSON.to_owned(),
                    payload: encoded,
                }),
                auth: Some(abi::AuthPlan {
                    kind: abi::AuthKind::Bearer as i32,
                    credential_slot: "api_key".to_owned(),
                    header_name: String::new(),
                }),
                ..Default::default()
            }),
        })
    }
}

impl chat::Attempt for OpenAiChatAttempt {
    fn transform_buffered(
        &mut self,
        upstream: abi::BufferedUpstreamResponse,
    ) -> sdk::DriverResult<chat::ClientResponse> {
        let head = upstream
            .head
            .ok_or_else(|| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
        let body = upstream
            .body
            .ok_or_else(|| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
        if self.closed || self.mode != abi::TextMode::Buffered {
            return Err(sdk::error(sdk::ERROR_DRIVER_INTERNAL));
        }
        if serde_json::from_slice::<Value>(&body.payload).is_err() {
            return Err(sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE));
        }
        let usage = usage_from_json(&body.payload);
        let headers = public_headers(head.headers);
        if (200..400).contains(&head.status_code) {
            let body = chat::decode_success(&body.payload)
                .map_err(|_| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
            Ok(chat::ClientResponse::new(head.status_code, headers, body).with_usage(usage))
        } else {
            let body = chat::decode_error(&body.payload)
                .map_err(|_| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
            Ok(chat::ClientResponse::new(head.status_code, headers, body).with_usage(usage))
        }
    }

    fn open_stream(
        &mut self,
        upstream: abi::UpstreamResponseHead,
    ) -> sdk::DriverResult<abi::TextSseOpenSuccess> {
        if self.closed || self.mode != abi::TextMode::Sse {
            return Err(sdk::error(sdk::ERROR_DRIVER_INTERNAL));
        }
        Ok(abi::TextSseOpenSuccess {
            status_code: upstream.status_code,
            headers: public_headers(upstream.headers),
            outcome: None,
        })
    }

    fn transform_stream_event(
        &mut self,
        upstream: abi::UpstreamSseEvent,
    ) -> sdk::DriverResult<chat::StreamEventResult> {
        if self.closed || self.mode != abi::TextMode::Sse || self.finished {
            return Err(sdk::error(sdk::ERROR_DRIVER_INTERNAL));
        }
        if upstream
            .data
            .iter()
            .copied()
            .filter(|byte| !byte.is_ascii_whitespace())
            .eq(b"[DONE]".iter().copied())
        {
            self.finished = true;
            return Ok(chat::StreamEventResult {
                events: vec![],
                outcome: Some(sdk::success()),
                usage: Some(sdk::unavailable_usage()),
            });
        }
        let event = chat::decode_event("chat.completion.chunk".to_owned(), &upstream.data)
            .map_err(|_| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
        Ok(chat::StreamEventResult {
            events: vec![event],
            outcome: None,
            usage: None,
        })
    }

    fn finish_stream(&mut self) -> sdk::DriverResult<chat::StreamFinishResult> {
        if self.closed {
            return Err(sdk::error(sdk::ERROR_DRIVER_INTERNAL));
        }
        if !self.finished {
            return Err(sdk::error(sdk::ERROR_UPSTREAM_STREAM_TRUNCATED));
        }
        Ok(chat::StreamFinishResult::default())
    }
    fn close(mut self) -> sdk::DriverResult<()> {
        self.closed = true;
        Ok(())
    }
}

fn usage_from_json(payload: &[u8]) -> abi::UsageReport {
    let value: Value = match serde_json::from_slice(payload) {
        Ok(value) => value,
        Err(_) => return sdk::unavailable_usage(),
    };
    let input = value
        .pointer("/usage/prompt_tokens")
        .and_then(Value::as_i64);
    let output = value
        .pointer("/usage/completion_tokens")
        .and_then(Value::as_i64);
    match (input, output) {
        (Some(input), Some(output)) => abi::UsageReport {
            status: abi::UsageStatus::Final as i32,
            input_tokens: Some(input),
            output_tokens: Some(output),
            provenance: abi::UsageProvenance::UpstreamReported as i32,
            ..Default::default()
        },
        _ => sdk::unavailable_usage(),
    }
}

fn public_headers(headers: Vec<abi::NameValues>) -> Vec<abi::NameValues> {
    headers
        .into_iter()
        .filter(|header| header.name.eq_ignore_ascii_case("retry-after"))
        .collect()
}

static GUEST: LazyLock<sdk::Guest<sdk::Dispatcher<OpenAiChatBinder>>> = LazyLock::new(|| {
    let dispatcher = sdk::Dispatcher::new(
        &[chat::CONTRACT],
        OpenAiChatBinder,
        vec![chat::register(OpenAiChatHandler)],
    )
    .expect("manifest and typed handlers must match");
    sdk::Guest::new(dispatcher)
});

#[no_mangle]
pub extern "C" fn legate_alloc_v1(size: u32) -> u32 {
    sdk::legate_sdk_alloc(size)
}
/// Releases an ABI input or output buffer.
///
/// # Safety
/// The pointer and size must describe one live SDK allocation.
#[no_mangle]
pub unsafe extern "C" fn legate_free_v1(pointer: u32, size: u32) {
    sdk::legate_sdk_free(pointer, size)
}

macro_rules! export_hook {
    ($name:ident, $handler:ident) => {
        /// Executes one Legate ABI hook.
        ///
        /// # Safety
        /// The pointer and size must describe one live SDK allocation.
        #[no_mangle]
        pub unsafe extern "C" fn $name(pointer: u32, size: u32) -> u64 {
            sdk::invoke(pointer, size, |input| GUEST.$handler(input))
        }
    };
}
export_hook!(legate_bind_v1, handle_bind);
export_hook!(legate_text_attempt_open_v1, handle_text_attempt_open);
export_hook!(
    legate_text_transform_buffered_response_v1,
    handle_text_transform_buffered_response
);
export_hook!(legate_text_sse_open_v1, handle_text_sse_open);
export_hook!(
    legate_text_sse_transform_event_v1,
    handle_text_sse_transform_event
);
export_hook!(legate_text_sse_finish_v1, handle_text_sse_finish);
export_hook!(legate_text_attempt_close_v1, handle_text_attempt_close);

#[cfg(test)]
mod tests {
    use super::*;
    use sdk::Driver;

    #[test]
    fn open_builds_request_without_prepare() {
        let dispatcher = sdk::Dispatcher::new(
            &[chat::CONTRACT],
            OpenAiChatBinder,
            vec![chat::register(OpenAiChatHandler)],
        )
        .expect("dispatcher");
        let result = dispatcher
            .open_text_attempt(abi::TextAttemptOpenRequest {
                bound_state: b"openai-chat-compatible-v1".to_vec(),
                invocation: Some(abi::TextInvocation {
                    mode: abi::TextMode::Buffered as i32,
                    request: Some(abi::ProtocolPayload {
                        protocol_contract: sdk::PROTOCOL_OPENAI_CHAT_COMPLETIONS_2026_07_18
                            .to_owned(),
                        media_type: sdk::MEDIA_TYPE_JSON.to_owned(),
                        json: br#"{"model":"public","messages":[]}"#.to_vec(),
                    }),
                    selected_upstream_model: "upstream".to_owned(),
                    response_id: "resp_1".to_owned(),
                    ..Default::default()
                }),
            })
            .expect("open");
        assert!(matches!(result.output, sdk::OpenAttemptOutput::Request(_)));
    }

    #[test]
    fn manifest_contracts_match_typed_handler_registrations() {
        let manifest: Value =
            serde_json::from_str(include_str!("../manifest.json")).expect("manifest");
        let dispatcher = sdk::Dispatcher::new(
            &[chat::CONTRACT],
            OpenAiChatBinder,
            vec![chat::register(OpenAiChatHandler)],
        )
        .expect("dispatcher");
        let declared = manifest["text"]["protocolContracts"]
            .as_array()
            .expect("protocol contracts")
            .iter()
            .map(|value| value.as_str().expect("contract").to_owned())
            .collect::<Vec<_>>();
        assert_eq!(declared, dispatcher.protocol_contracts());
    }
}
