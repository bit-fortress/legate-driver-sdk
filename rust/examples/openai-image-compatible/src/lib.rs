use legate_driver_sdk as sdk;
use sdk::abi;
use sdk::protocol::openai::images::edits::v20260719 as edit;
use sdk::protocol::openai::images::generations::v20260719 as generation;
use std::sync::LazyLock;

struct Binder;
struct GenerationHandler;
struct EditHandler;
struct GenerationAttempt;
struct EditAttempt;

impl sdk::ImageBinder for Binder {
    fn bind_image(&self, request: abi::BindRequest) -> sdk::DriverResult<Vec<u8>> {
        if !sdk::is_empty_json_object(&request.config_json)
            || !request.endpoint_refs.iter().any(|value| value == "primary")
            || !request
                .credential_slots
                .iter()
                .any(|slot| slot.name == "api_key" && slot.configured)
        {
            return Err(sdk::error(sdk::ERROR_INVALID_CONFIG));
        }
        Ok(b"openai-image-compatible-v1".to_vec())
    }
}

impl generation::Handler for GenerationHandler {
    type Attempt = GenerationAttempt;
    fn open_attempt(
        &self,
        mut input: generation::OpenAttemptInput,
    ) -> sdk::DriverResult<generation::OpenAttempt<Self::Attempt>> {
        if input.bound_state != b"openai-image-compatible-v1" {
            return Err(sdk::error(sdk::ERROR_INVALID_INVOCATION));
        }
        input.request.model = input.selected_upstream_model;
        let body = generation::encode_request(&input.request)
            .map_err(|_| sdk::error(sdk::ERROR_DRIVER_INTERNAL))?;
        Ok(generation::OpenAttempt {
            attempt: GenerationAttempt,
            output: generation::OpenAttemptOutput::Request(request_plan(
                "/images/generations",
                sdk::inline_body(sdk::MEDIA_TYPE_JSON, body),
            )),
        })
    }
}

impl generation::Attempt for GenerationAttempt {
    fn transform_buffered(
        &mut self,
        upstream: abi::ImageUpstreamResponse,
    ) -> sdk::DriverResult<generation::ClientResponse> {
        let head = upstream
            .head
            .ok_or_else(|| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
        let body = upstream
            .body
            .and_then(|body| body.content)
            .ok_or_else(|| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
        let response = match body {
            abi::image_upstream_body::Content::Blob(value) => generation::ClientResponse::new(
                head.status_code,
                public_headers(head.headers),
                value,
            ),
            abi::image_upstream_body::Content::Inline(value)
                if (200..400).contains(&head.status_code) =>
            {
                let value = generation::decode_success(&value)
                    .map_err(|_| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
                generation::ClientResponse::new(
                    head.status_code,
                    public_headers(head.headers),
                    value,
                )
            }
            abi::image_upstream_body::Content::Inline(value) => {
                let value = generation::decode_error(&value)
                    .map_err(|_| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
                generation::ClientResponse::new(
                    head.status_code,
                    public_headers(head.headers),
                    value,
                )
            }
        };
        Ok(response.with_usage(sdk::unavailable_usage()))
    }
    fn close(self) -> sdk::DriverResult<()> {
        Ok(())
    }
}

impl edit::Handler for EditHandler {
    type Attempt = EditAttempt;
    fn open_attempt(
        &self,
        input: edit::OpenAttemptInput,
    ) -> sdk::DriverResult<edit::OpenAttempt<Self::Attempt>> {
        if input.bound_state != b"openai-image-compatible-v1" {
            return Err(sdk::error(sdk::ERROR_INVALID_INVOCATION));
        }
        let body = input
            .request
            .multipart_body(&input.selected_upstream_model)
            .map_err(|_| sdk::error(sdk::ERROR_DRIVER_INTERNAL))?;
        Ok(edit::OpenAttempt {
            attempt: EditAttempt,
            output: edit::OpenAttemptOutput::Request(request_plan("/images/edits", body)),
        })
    }
}

impl edit::Attempt for EditAttempt {
    fn transform_buffered(
        &mut self,
        upstream: abi::ImageUpstreamResponse,
    ) -> sdk::DriverResult<edit::ClientResponse> {
        let head = upstream
            .head
            .ok_or_else(|| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
        let body = upstream
            .body
            .and_then(|body| body.content)
            .ok_or_else(|| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
        let response = match body {
            abi::image_upstream_body::Content::Blob(value) => {
                edit::ClientResponse::new(head.status_code, public_headers(head.headers), value)
            }
            abi::image_upstream_body::Content::Inline(value)
                if (200..400).contains(&head.status_code) =>
            {
                let value = generation::decode_success(&value)
                    .map_err(|_| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
                edit::ClientResponse::new(head.status_code, public_headers(head.headers), value)
            }
            abi::image_upstream_body::Content::Inline(value) => {
                let value = generation::decode_error(&value)
                    .map_err(|_| sdk::error(sdk::ERROR_MALFORMED_UPSTREAM_RESPONSE))?;
                edit::ClientResponse::new(head.status_code, public_headers(head.headers), value)
            }
        };
        Ok(response.with_usage(sdk::unavailable_usage()))
    }
    fn close(self) -> sdk::DriverResult<()> {
        Ok(())
    }
}

fn request_plan(path: &str, body: abi::BodyPlan) -> abi::RequestPlan {
    abi::RequestPlan {
        endpoint_ref: "primary".to_owned(),
        method: "POST".to_owned(),
        relative_path: path.to_owned(),
        body_plan: Some(body),
        auth: Some(abi::AuthPlan {
            kind: abi::AuthKind::Bearer as i32,
            credential_slot: "api_key".to_owned(),
            header_name: String::new(),
        }),
        ..Default::default()
    }
}

fn public_headers(headers: Vec<abi::NameValues>) -> Vec<abi::NameValues> {
    headers
        .into_iter()
        .filter(|header| {
            !header.name.eq_ignore_ascii_case("content-type")
                && !header.name.eq_ignore_ascii_case("content-length")
        })
        .collect()
}

static GUEST: LazyLock<sdk::ImageGuest<sdk::ImageDispatcher<Binder>>> = LazyLock::new(|| {
    let dispatcher = sdk::ImageDispatcher::new(
        &[generation::CONTRACT, edit::CONTRACT],
        Binder,
        vec![
            generation::register(GenerationHandler),
            edit::register(EditHandler),
        ],
    )
    .expect("manifest and typed image handlers must match");
    sdk::ImageGuest::new(dispatcher)
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
export_hook!(legate_image_attempt_open_v1, handle_image_attempt_open);
export_hook!(
    legate_image_transform_buffered_response_v1,
    handle_image_transform_buffered_response
);
export_hook!(legate_image_attempt_close_v1, handle_image_attempt_close);
