FROM golang:1.21.1-bullseye as build

WORKDIR /app

ARG GO_ARGS=""

COPY go.* /app/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=ssh \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build $GO_ARGS -o /app/outbin

# Need glibc
FROM gcr.io/distroless/base-debian11

ENTRYPOINT ["/app/outbin"]
COPY --from=build /app/outbin /app/
