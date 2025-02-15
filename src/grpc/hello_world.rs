use futures::stream::StreamExt;
use prost::Message;
use tokio_stream::wrappers::ReceiverStream;
use tonic::{Request, Response, Status};

pub mod helloworld {
    tonic::include_proto!("helloworld");
}

use helloworld::greeter_server::Greeter;
use helloworld::{HelloReply, HelloRequest};
use tracing::debug;

#[derive(Default, Debug)]
pub struct MyGreeter {}

#[tonic::async_trait]
impl Greeter for MyGreeter {
    async fn say_hello(
        &self,
        request: Request<HelloRequest>, // Accept request of type HelloRequest
    ) -> Result<Response<HelloReply>, Status> {
        // Return an instance of type HelloReply
        debug!("Got a request: {:?}", request);

        let req_inner = request.into_inner();

        let reply = HelloReply {
            message: format!("Hello {}!", req_inner.name), // We must use .into_inner() as the fields of gRPC requests and responses are private
        };

        let bytes = req_inner.encode_to_vec();
        let reconstructed = HelloRequest::decode(&mut &bytes[..]).unwrap();
        println!("Reconstructed: {:?}", reconstructed);

        Ok(Response::new(reply)) // Send back our formatted greeting
    }

    type StreamHelloStream = ReceiverStream<Result<HelloReply, Status>>;

    async fn stream_hello(
        &self,
        request: Request<tonic::Streaming<HelloRequest>>,
    ) -> Result<Response<Self::StreamHelloStream>, Status> {
        let mut stream = request.into_inner();
        let (tx, rx) = tokio::sync::mpsc::channel(4);

        // Spawn a task to process the incoming stream
        let mut i = 0;
        tokio::spawn(async move {
            while let Some(req) = stream.next().await {
                if let Ok(request) = req {
                    println!("Sending message {} to channel", i);
                    tx.send(Ok(HelloReply {
                        message: format!("Hello {}! - {}", request.name, i),
                    }))
                    .await
                    .unwrap();
                    i += 1;
                }
            }
        });

        Ok(Response::new(ReceiverStream::new(rx)))
    }
}
