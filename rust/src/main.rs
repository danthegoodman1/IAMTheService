use rust_http_template::start;
use tokio::signal;
use tracing::{debug, warn, Level};
use tracing_subscriber::{fmt::format::FmtSpan, layer::SubscriberExt, Layer};

#[tokio::main]
async fn main() {
    // tracing_subscriber::fmt::init();
    let subscriber = tracing_subscriber::registry().with(
        tracing_subscriber::fmt::layer()
            .compact()
            // .json()
            .with_file(true)
            .with_line_number(true)
            .with_span_events(FmtSpan::CLOSE)
            .with_target(false)
            .with_filter(
                tracing_subscriber::filter::Targets::new()
                .with_target("h2", Level::INFO) // filter out h2 logs
                .with_target("tower", Level::INFO) // filter out tower debug logs
                .with_default(Level::DEBUG),
            ),
    );

    tracing::subscriber::set_global_default(subscriber).unwrap();

    tokio::select! {
        _ = start("0.0.0.0:8080") => {},
        _ = shutdown_signal() => {
            warn!("Shutdown timer completed, terminating...");
        }
    }
}

async fn shutdown_signal() {
    signal::ctrl_c().await.expect("Failed to listen for ctrl-c");
    debug!("Received Ctrl-C, starting 30s shutdown timer...");
    tokio::time::sleep(std::time::Duration::from_secs(30)).await;
}
