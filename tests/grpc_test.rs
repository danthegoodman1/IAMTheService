use futures::stream::{self, StreamExt};
use rust_http_template::grpc::hello_world::helloworld::{
    greeter_client::GreeterClient, HelloRequest,
};
use tonic::Request;

#[tokio::test]
async fn test_grpc_hello() -> Result<(), Box<dyn std::error::Error>> {
    // Create a gRPC client
    let mut client = GreeterClient::connect("http://0.0.0.0:50051").await?;

    // Create a new request
    let request = tonic::Request::new(HelloRequest {
        name: "World".to_string(),
    });

    // Send the request and await the response
    let response = client.say_hello(request).await?;
    assert_eq!(response.get_ref().message, "Hello World!");
    println!("RESPONSE={:?}", response);

    Ok(())
}

#[tokio::test]
async fn test_grpc_stream_hello() -> Result<(), Box<dyn std::error::Error>> {
    // Create a gRPC client
    let mut client = GreeterClient::connect("http://0.0.0.0:50051").await?;

    // Create a stream of requests
    let requests = vec![
        HelloRequest {
            name: "Alice".to_string(),
        },
        HelloRequest {
            name: "Bob".to_string(),
        },
        HelloRequest {
            name: "Charlie".to_string(),
        },
    ];

    let request_stream = stream::iter(requests);
    let response_stream = client.stream_hello(Request::new(request_stream)).await?;
    let mut responses = response_stream.into_inner();

    // Collect and verify responses
    let mut i = 0;
    while let Some(response) = responses.next().await {
        let response = response?;
        println!("Received message {}", response.message);
        assert_eq!(
            response.message,
            match i {
                0 => "Hello Alice! - 0",
                1 => "Hello Bob! - 1",
                2 => "Hello Charlie! - 2",
                _ => panic!("Unexpected response"),
            }
        );
        i += 1;
    }

    assert_eq!(i, 3, "Should have received exactly 3 responses");
    Ok(())
}
