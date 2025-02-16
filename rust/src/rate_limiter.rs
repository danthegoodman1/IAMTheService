use axum::extract::Request;
use axum::extract::State;
use axum::middleware::Next;
use axum::response::Response;
use std::collections::HashMap;
use std::time::{Duration, Instant};
use tokio::sync::Mutex;

use crate::AppState;

pub struct RateLimiter {
    requests: Mutex<HashMap<String, (u64, Instant)>>,
    max_requests: u64,
    window: Duration,
}

impl RateLimiter {
    pub fn new(max_requests: u64, window: Duration) -> Self {
        Self {
            requests: Mutex::new(HashMap::new()),
            max_requests,
            window,
        }
    }

    pub async fn check(&self, key: &str) -> (u64, bool) {
        let mut requests = self.requests.lock().await;
        let now = Instant::now();

        let (current_count, current_time) = requests
            .get(key)
            .map(|(count, time)| (*count, *time))
            .unwrap_or((0, now));

        if now.duration_since(current_time) > self.window {
            requests.insert(key.to_string(), (1, now));
            (1, true)
        } else if current_count >= self.max_requests {
            (current_count, false)
        } else {
            let new_count = current_count + 1;
            requests.insert(key.to_string(), (new_count, current_time));
            (new_count, true)
        }
    }
}

pub async fn ip_rate_limiter(
    State(state): State<AppState>,
    request: Request,
    next: Next,
) -> Response {
    // Get the client's IP address from the request
    let ip = request
        .headers()
        .get("x-forwarded-for")
        .and_then(|hv| hv.to_str().ok())
        .unwrap_or("unknown");

    // Check if the request is allowed
    let (_, allowed) = state.rate_limiter.check(ip).await;

    if !allowed {
        // Return 429 Too Many Requests if rate limit exceeded
        Response::builder()
            .status(429)
            .body(axum::body::Body::from("Too Many Requests"))
            .unwrap()
    } else {
        // Continue to next handler if allowed
        next.run(request).await
    }
}
