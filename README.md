# A Protobuf parser for Go

This package contains a cleanroom Protobuf parser for Go using [Participle](https://github.com/alecthomas/participle).

This was originally an example within Participle.

## Tests

Conformance tests are pulled from protoc and can be run with `go test ./...`. 
You can re-sync the upstream tests by running `make -C testdata`.

Compiler tests are end to end tests comparing generated FileDesciptors
against protoc generated FileDescriptors. The protoc generated
FileDescriptors are located in `compiler/testdata/pb/*.pb` and
source files in `compiler/testdata/*.proto`. Protoc FileDescriptors can be
regenerated with `make -C compiler`
