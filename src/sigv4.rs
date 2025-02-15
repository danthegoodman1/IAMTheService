use axum::{
    extract::State,
    http::{Request, Response, StatusCode},
    response::IntoResponse,
    body::Body,
};
use futures::TryStreamExt;
use hmac::{Hmac, Mac};
use sha2::{Digest, Sha256};
use std::fmt::Write;
use anyhow::Error;
use hex;
use reqwest;

use crate::AppState;

// max payload size is 256k
type HmacSha256 = Hmac<Sha256>;

fn get_hmac(key: &[u8], data: &[u8]) -> Vec<u8> {
    let mut mac = HmacSha256::new_from_slice(key)
        .expect("HMAC can take key of any size");
    mac.update(data);
    let result = mac.finalize();
    result.into_bytes().to_vec()
}

fn get_sha256(data: &[u8]) -> Vec<u8> {
    let mut hasher = Sha256::new();
    hasher.update(data);
    hasher.finalize().to_vec()
}

fn get_string_to_sign(req: &Request<Body>, canonical_request: &str) -> String {
    let mut s = String::from("AWS4-HMAC-SHA256\n");

    let x_amz_date = req.headers()
        .get("X-Amz-Date")
        .and_then(|value| value.to_str().ok())
        .unwrap_or_default();

    s.push_str(x_amz_date);
    s.push('\n');

    let scope = format!("{}/{}/{}/{}", &x_amz_date[..8], "us-east-1", "dynamodb", "aws4_request");
    s.push_str(&scope);
    s.push('\n');

    let canonical_request_hash = get_sha256(canonical_request.as_bytes());
    let mut hex_encoded_hash = String::new();
    for byte in canonical_request_hash {
        write!(hex_encoded_hash, "{:02x}", byte)
            .expect("Can write to a String");
    }

    s.push_str(&hex_encoded_hash);
    s
}

fn get_signing_key(req: &Request<Body>) -> Vec<u8> {
    let x_amz_date = req.headers()
        .get("X-Amz-Date")
        .and_then(|value| value.to_str().ok())
        .unwrap_or_default();

    let date_key = get_hmac(b"AWS4testpassword", &x_amz_date[..8].as_bytes());
    let date_region_key = get_hmac(&date_key, b"us-east-1");
    let date_region_service_key = get_hmac(&date_region_key, b"dynamodb");
    let signing_key = get_hmac(&date_region_service_key, b"aws4_request");
    signing_key
}

struct AWSAuthHeaderCredential {
    key_id: String,
    date: String,
    region: String,
    service: String,
    request: String,
}

struct AWSAuthHeader {
    credential: AWSAuthHeaderCredential,
    signed_headers: Vec<String>,
    signature: String,
}

fn get_aws_auth_header(req: &Request<Body>) -> Result<AWSAuthHeader, Error> {
    let mut auth_header = AWSAuthHeader {
        signature: String::new(),
        credential: AWSAuthHeaderCredential {
            date: String::new(),
            key_id: String::new(),
            region: String::new(),
            request: String::new(),
            service: String::new(),
        },
        signed_headers: Vec::new(),
    };

    // TODO: make this more efficient, optional host override
    // Extract signed headers and other parts from the Authorization header.
    if let Some(header_value) = req.headers().get("Authorization") {
        let header_str = header_value
            .to_str()
            .expect("failed to parse auth header to string");
        for item in header_str.split_whitespace() {
            let item = item.trim_end_matches(",");
            if item.starts_with("SignedHeaders=") {
                let headers = item
                    .trim_start_matches("SignedHeaders=")
                    .replace(",", ";");
                auth_header.signed_headers =
                    headers.split(';').map(str::to_string).collect();
            }
            if item.starts_with("Credential=") {
                let credential_parts: Vec<String> = item
                    .trim_start_matches("Credential=")
                    .split('/')
                    .map(str::to_string)
                    .collect();
                if credential_parts.len() >= 5 {
                    auth_header.credential = AWSAuthHeaderCredential {
                        key_id: credential_parts[0].clone(),
                        date: credential_parts[1].clone(),
                        region: credential_parts[2].clone(),
                        service: credential_parts[3].clone(),
                        request: credential_parts[4].clone(),
                    };
                }
            }
            if item.starts_with("Signature=") {
                auth_header.signature =
                    item.trim_start_matches("Signature=").to_string();
            }
        }
    }
    Ok(auth_header)
}

fn get_canonical_request(req: &Request<Body>, auth_header: &AWSAuthHeader) -> Result<String, Error> {
    let mut canonical_request = String::new();

    // Add HTTP method.
    canonical_request.push_str(req.method().as_str());
    canonical_request.push('\n');

    // Add the path.
    canonical_request.push_str(req.uri().path());
    canonical_request.push('\n');

    // Add the encoded query string.
    let query_string = req.uri().query().unwrap_or_default();
    canonical_request.push_str(query_string);
    canonical_request.push('\n');

    // Add headers to canonical request.
    for header_name in &auth_header.signed_headers {
        canonical_request.push_str(header_name);
        canonical_request.push(':');
        if let Some(val) = req.headers().get(header_name) {
            canonical_request.push_str(val.to_str().unwrap_or(""));
        }
        canonical_request.push('\n');
    }

    // Add newline separator.
    canonical_request.push('\n');

    // Add signed headers names.
    canonical_request.push_str(&auth_header.signed_headers.join(";"));
    canonical_request.push('\n');

    // Handle 'x-amz-content-sha256' header.
    let sha_header = req.headers().get("x-amz-content-sha256").map_or_else(
        || "UNSIGNED-PAYLOAD".to_string(),
        |h| h.to_str().unwrap_or("UNSIGNED-PAYLOAD").to_owned(),
    );
    canonical_request.push_str(&sha_header);
    Ok(canonical_request)
}

fn extract_provided_signature(req: &Request<Body>) -> Option<String> {
    let authorization_header = req.headers().get("Authorization")?.to_str().ok()?;
    let parts: Vec<&str> = authorization_header.split(", ").collect();
    for item in parts {
        if item.starts_with("Signature") {
            return item.split('=').nth(1).map(|s| s.to_string());
        }
    }
    None
}

#[axum::debug_handler]
pub async fn proxy_request(
    State(state): State<AppState>,
    req: Request<Body>,
) -> Result<impl IntoResponse, (StatusCode, String)> {
    let parsed_auth_header =
        get_aws_auth_header(&req).map_err(|e| (StatusCode::BAD_REQUEST, e.to_string()))?;
    let canonical_request =
        get_canonical_request(&req, &parsed_auth_header).map_err(|e| (StatusCode::BAD_REQUEST, e.to_string()))?;
    let string_to_sign = get_string_to_sign(&req, &canonical_request);
    let signing_key = get_signing_key(&req);
    let signature =
        hex::encode(get_hmac(&signing_key, string_to_sign.as_bytes()));
    let provided_signature =
        extract_provided_signature(&req).unwrap_or_else(|| "None".to_string());
    println!("Provided Signature: {}", provided_signature);
    println!("Calculated Signature: {}", signature);

    // Define the backend URL to proxy to.
    let base_url = "https://httpbin.org/anything";
    let uri = req.uri();
    let url = if let Some(query) = uri.query() {
        format!("{}?{}", base_url, query)
    } else {
        base_url.to_string()
    };

    // Clone headers and method before consuming the body.
    let headers = req.headers().clone();
    let method = req.method().clone();


    let old_host = url::Url::parse(&url).unwrap().host().unwrap();

    // TODO: Resign the request with the new host

    // Convert the Axum body into a stream and map its error type.
    let body_stream = req.into_body().into_data_stream();
    let proxied_body = reqwest::Body::wrap_stream(body_stream.into_stream());

    // Build the proxied request using the Reqwest client with streaming body.
    let client_req = state.client
        .request(method, &url)
        .headers(headers)
        .body(proxied_body);

    let response = client_req
        .send()
        .await
        .map_err(|e| (StatusCode::BAD_GATEWAY, e.to_string()))?;

    // Build an Axum response from the Reqwest response.
    let status = response.status();
    let mut response_builder = Response::builder().status(status);
    for (key, value) in response.headers().iter() {
        response_builder = response_builder.header(key, value);
    }

    // TODO: check if we want to intercept the body

    // Stream the response back to the client.
    let stream = response.bytes_stream();

    Ok(axum::response::Response::builder()
    .header("Content-Type", "application/octet-stream")
    .body(axum::body::Body::from_stream(stream))
    .unwrap())
}
