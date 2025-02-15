use futures_util::stream::StreamExt;
use reqwest::Client;
use rust_http_template::json_rpc::{JsonRpcRequest, JsonRpcResponseError, JsonRpcResponseSuccess};
use serde::{Deserialize, Serialize};
use serde_json::json;

const BASE_URL: &str = "http://localhost:8080";

#[tokio::test]
async fn test_echo_json() {
    let client = Client::new();
    let test_json = json!({
        "message": "hello",
        "number": 42
    });

    let response = client
        .post(format!("{}/echo/json", BASE_URL))
        .json(&test_json)
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let response_json = response.json::<serde_json::Value>().await.unwrap();
    assert_eq!(response_json, test_json);
}

#[tokio::test]
async fn test_sse_endpoint() {
    let client = Client::new();
    let response = client
        .get(format!("{}/sse", BASE_URL))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    assert_eq!(
        response.headers().get("content-type").unwrap(),
        "text/event-stream"
    );
    assert_eq!(response.headers().get("content-length"), None); // No content length because it's a stream

    let mut body = response.bytes_stream();
    let mut accumulated = String::new();

    while let Some(chunk) = body.next().await {
        let chunk = chunk.unwrap();
        let text = String::from_utf8_lossy(&chunk);
        println!("Raw chunk: {:?}", text);
        // Process each line separately
        for line in text.lines() {
            if let Some(content) = line.strip_prefix("data: ") {
                println!("Stripped content: {:?}", content);
                accumulated.push_str(content);
            }
        }
    }

    println!("Final accumulated: {:?}", accumulated);
    assert!(accumulated.contains("Hello, World!"));
}

#[tokio::test]
async fn test_stream_endpoint() {
    let client = Client::new();
    let response = client
        .get(format!("{}/stream", BASE_URL))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    assert_eq!(
        response.headers().get("content-type").unwrap(),
        "application/octet-stream"
    );
    assert_eq!(response.headers().get("content-length"), None); // No content length because it's a stream

    let body = response.text().await.unwrap();
    assert_eq!(body, "Hello, World!\n");
}

#[tokio::test]
async fn test_echo_json_extractor() {
    let client = Client::new();

    // Test valid payload
    let valid_payload = json!({
        "name": "Alice"
    });
    let response = client
        .post(format!("{}/echo/json_extractor", BASE_URL))
        .json(&valid_payload)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 200);
    let response_json = response.json::<serde_json::Value>().await.unwrap();
    assert_eq!(response_json, valid_payload);

    // Test invalid payload (name too short)
    let invalid_payload = json!({
        "name": "Ab"
    });
    let response = client
        .post(format!("{}/echo/json_extractor", BASE_URL))
        .json(&invalid_payload)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 400); // Unprocessable Entity
    println!("Response body: {:?}", response.text().await.unwrap());

    // Test invalid payload (name too long)
    let invalid_payload = json!({
        "name": "ThisNameIsTooLong"
    });
    let response = client
        .post(format!("{}/echo/json_extractor", BASE_URL))
        .json(&invalid_payload)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 400);
    println!("Response body: {:?}", response.text().await.unwrap());
}

#[tokio::test]
async fn test_stream_handler() {
    let client = Client::new();
    let test_data = "Hello from the stream!";

    let response = client
        .post(format!("{}/stream_handler", BASE_URL))
        .body(test_data.to_string())
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let body = response.text().await.unwrap();
    assert_eq!(body, test_data);
}

// Copied because they're not exported from the module
#[derive(Debug, Serialize, Deserialize)]
pub struct MyRpcParams {
    pub name: String,
}

#[derive(Debug, Serialize)]
pub struct MyRpcResponse {
    pub message: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct GreetingRpcParams {
    pub name: String,
    pub language: String,
}

#[derive(Debug, Serialize)]
pub struct GreetingRpcResponse {
    pub greeting: String,
    pub translated: bool,
}

#[tokio::test]
async fn test_json_rpc_my_rpc() {
    let client = Client::new();
    let payload = JsonRpcRequest {
        jsonrpc: "2.0".to_string(),
        id: Some(1),
        method: "my_rpc".to_string(),
        params: serde_json::to_value(MyRpcParams {
            name: "Alice".to_string(),
        })
        .unwrap(),
    };

    let response = client
        .post(format!("{}/json_rpc", BASE_URL))
        .json(&payload)
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let response_json = response.json::<serde_json::Value>().await.unwrap();

    let expected: JsonRpcResponseSuccess<MyRpcResponse> = JsonRpcResponseSuccess {
        jsonrpc: "2.0".to_string(),
        id: Some(1),
        result: MyRpcResponse {
            message: "Hello, Alice!".to_string(),
        }
        .into(),
    };
    assert_eq!(response_json, serde_json::to_value(expected).unwrap());

    // Test error case
    let error_payload = JsonRpcRequest {
        jsonrpc: "2.0".to_string(),
        id: Some(1),
        method: "my_rpc".to_string(),
        params: serde_json::to_value(MyRpcParams {
            name: "error".to_string(),
        })
        .unwrap(),
    };

    let response = client
        .post(format!("{}/json_rpc", BASE_URL))
        .json(&error_payload)
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let response_json = response.json::<serde_json::Value>().await.unwrap();

    let expected_error = JsonRpcResponseError {
        jsonrpc: "2.0".to_string(),
        id: Some(1),
        data: Some(json!({
            "error": "Internal error"
        })),
        code: -32603,
    };

    assert_eq!(response_json, serde_json::to_value(expected_error).unwrap());
}

#[tokio::test]
async fn test_json_rpc_greeting_rpc() {
    let client = Client::new();

    // Test different languages
    let test_cases = vec![
        ("Spanish", "¡Hola, Bob!", true),
        ("English", "Hello, Bob!", false),
        ("French", "Bonjour, Bob!", true),
    ];

    for (language, expected_greeting, expected_translated) in test_cases {
        let payload = JsonRpcRequest {
            jsonrpc: "2.0".to_string(),
            id: Some(1),
            method: "greeting_rpc".to_string(),
            params: serde_json::to_value(GreetingRpcParams {
                name: "Bob".to_string(),
                language: language.to_string(),
            })
            .unwrap(),
        };

        let response = client
            .post(format!("{}/json_rpc", BASE_URL))
            .json(&payload)
            .send()
            .await
            .unwrap();

        assert_eq!(response.status(), 200);
        let response_json = response.json::<serde_json::Value>().await.unwrap();

        let expected: JsonRpcResponseSuccess<GreetingRpcResponse> = JsonRpcResponseSuccess {
            jsonrpc: "2.0".to_string(),
            id: Some(1),
            result: GreetingRpcResponse {
                greeting: expected_greeting.to_string(),
                translated: expected_translated,
            }
            .into(),
        };
        assert_eq!(response_json, serde_json::to_value(expected).unwrap());
    }
}
