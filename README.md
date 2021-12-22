# A Protobuf parser for Go

This package contains a cleanroom Protobuf parser for Go using [Participle](https://github.com/alecthomas/participle).

This was originally an example within Participle.

## Tests

Conformance tests are pulled from protoc and can be run with `go test -tags
conformance ./...`. These are currently failing and will be enabled by default
once passing. You can re-sync the upstream tests by running `make -C testdata`.
