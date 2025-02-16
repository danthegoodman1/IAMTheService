# Build stage
FROM rust:1.82.0-bullseye as builder

WORKDIR /app

# Copy over manifests AND source code first
COPY . .

# Now build for release
RUN cargo build --release

# Final stage
FROM gcr.io/distroless/cc-debian12

# Copy the built binary from builder
COPY --from=builder /app/target/release/citizenstats_server /app/citizenstats_server

# Set the binary as the entrypoint
ENTRYPOINT ["/app/citizenstats_server"]
