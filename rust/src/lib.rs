use axum::routing::any;
use axum::middleware;
use hyper::body::Body;

use std::sync::Arc;
use std::time::Duration;

use axum::{
    error_handling::HandleErrorLayer,
    extract::DefaultBodyLimit,
    http::StatusCode,
    response::{IntoResponse, Response},
};
use tower::{buffer::BufferLayer, BoxError, ServiceBuilder};
use tracing::{error, info, info_span, Instrument};

mod rate_limiter;
mod sigv4;
use rate_limiter::{ip_rate_limiter, RateLimiter};

#[derive(Clone)]
struct AppState {
    rate_limiter: Arc<RateLimiter>,
    client: reqwest::Client,
}

pub async fn start(http_addr: &str) {
    let state = AppState {
        rate_limiter: Arc::new(RateLimiter::new(10, Duration::from_secs(60))), // 10 requests per minute
        client: reqwest::Client::new(),
    };

    let app = axum::Router::new()
        .route("/", any(sigv4::proxy_request))
        // .route(
        //     "/{key}",
        //     get(routes::get::get_key).post(routes::post::write_key),
        // )
        .layer(
            ServiceBuilder::new()
                // https://github.com/tokio-rs/axum/discussions/987
                .layer(HandleErrorLayer::new(|err: BoxError| async move {
                    // turns layer errors into HTTP errors
                    error!("Unhandled error: {}", err);
                    (
                        StatusCode::INTERNAL_SERVER_ERROR,
                        format!("Unhandled error: {}", err),
                    )
                }))
                .layer(BufferLayer::new(1024))
                .layer(DefaultBodyLimit::max(1_000_000))
                // also see https://docs.rs/tower-http/0.6.1/tower_http/request_id/index.html#example
                .layer(tower::timeout::TimeoutLayer::new(Duration::from_secs(60))) // 30 second timeout
                .layer(middleware::from_fn_with_state(
                    state.clone(),
                    ip_rate_limiter,
                )),
        )
        .with_state(state);

    info!("Starting on http://{}", http_addr);
    let axum_listener = tokio::net::TcpListener::bind(http_addr).await.unwrap();

    let axum_server = axum::serve(axum_listener, app).with_graceful_shutdown(async {
        tokio::signal::ctrl_c()
            .await
            .expect("Failed to install Ctrl+C handler");
        info!("Received shutdown signal");
    });

    _ = tokio::join!(axum_server);
}

// Make our own error that wraps `anyhow::Error`.
pub enum AppError {
    Anyhow(anyhow::Error),
    CustomCode(anyhow::Error, axum::http::StatusCode),
    RateLimited(anyhow::Error),
    ValidationError(validator::ValidationErrors),
}

// Tell axum how to convert `AppError` into a response.
impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        match self {
            AppError::Anyhow(e) => (
                StatusCode::INTERNAL_SERVER_ERROR,
                format!("Something went wrong: {}", e),
            ),
            AppError::CustomCode(e, code) => (code, format!("{}", e)),
            AppError::RateLimited(e) => (
                StatusCode::TOO_MANY_REQUESTS,
                format!("Rate limit exceeded: {}", e),
            ),
            AppError::ValidationError(e) => {
                (StatusCode::BAD_REQUEST, format!("Validation error: {}", e))
            }
        }
        .into_response()
    }
}

// This enables using `?` on functions that return `Result<_, anyhow::Error>` to turn them into
// `Result<_, AppError>`. That way you don't need to do that manually.
impl<E> From<E> for AppError
where
    E: Into<anyhow::Error>,
{
    fn from(err: E) -> Self {
        Self::Anyhow(err.into())
    }
}

impl AppError {
    pub fn rate_limited() -> Self {
        Self::CustomCode(
            anyhow::anyhow!("Rate limit exceeded"),
            StatusCode::TOO_MANY_REQUESTS,
        )
    }
}

/// This middleware adds a request id to the span, and logs the path, request body size, and response size.
/// Uses tracing gymnastics... if this span is not included then the req_id is not propagated.
pub async fn trace_http(req: axum::extract::Request, next: axum::middleware::Next) -> Response {
    // Extract HTTP method and URI path.
    let method = req.method().clone();
    let path = req.uri().path().to_owned();

    // Attempt to read the request body size from the "Content-Length" header.
    let req_body_size = req
        .headers()
        .get("content-length")
        .and_then(|h| h.to_str().ok())
        .unwrap_or("?");

    // Use the provided request id or generate a new one if none is present.
    let req_id = req
        .headers()
        .get("X-Request-ID")
        .and_then(|h| h.to_str().ok())
        .map(|s| s.to_string())
        .unwrap_or_else(|| uuid::Uuid::new_v4().to_string());

    // Create a tracing span that includes our custom fields.
    let span = info_span!(
        target: "req_handler",
        "req_handler",
        req_id = %req_id, // % means display formatting
        method = %method,
        path = %path,
        req_size = %req_body_size,
        res_size = tracing::field::Empty,
    );

    // wrap it so we can record the response body size in the span
    let response = async {
        let response = next.run(req).await;
        // Try extracting the response body size from the "Content-Length" header.
        let res_body_size: String = response
            .size_hint()
            .upper()
            .and_then(|s| Some(s.to_string()))
            .unwrap_or("?".to_string());
        span.record("res_size", &format_args!("{}", res_body_size)); // prevent debug formatting
        response
    }
    .instrument(span.clone())
    .await;

    response
}
