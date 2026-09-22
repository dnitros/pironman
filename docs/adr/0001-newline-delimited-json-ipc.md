# Use newline-delimited JSON for the CLI-daemon IPC protocol

The CLI and daemon communicate over a Unix domain socket. We considered length-prefixed binary framing (protobuf/msgpack) and an RPC framework (`net/rpc`, `jsonrpc2`, gRPC), but chose newline-delimited JSON request/response instead. Traffic is low-frequency control commands (e.g. `rgb color #ff0000`), not high-throughput streaming, so simplicity and debuggability (inspectable with `socat`/`nc`, no codegen step) outweigh the marginal efficiency a binary protocol would buy.
