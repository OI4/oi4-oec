# OEC demo services (P6)

Three small services show the OEC SDKs at work (ADR-0028):

| Demo | Language, SDK | ServiceType | Messaging level | State |
|---|---|---|---|---|
| [`logger/`](logger/README.md) – D4 | C17 on the C ABI (`oec-ffi`) | `Utility` | L2 | core since C-079, no Web UI yet |
| `weather/` – D3 | Go on the Go SDK | `OTConnector` | L3 | planned (C-080) |
| `config/` – D10 | Rust on `oec` | `Utility` | L1 | planned (C-082) |

Each demo gets a small Web UI served by the service itself (static HTML/CSS/JS,
Server-Sent Events) on `127.0.0.1` only and **without authentication** – a demo,
not a product (increment 5). Images and the compose profile `demos` of
`oec/dev` follow in increment 6.

## Layout

```text
oec/demos/
├── check.sh             gate step (called by oec/check.sh)
├── go.mod               one Go module, standard library only
├── internal/demotest/   test harness: test broker, container storage, process, D7
├── reqs_test.go         every REQ id cited here exists and is active
└── logger/              D4: src/ and test/ in C, Makefile, Go integration tests
```

## Tests

Everything runs offline, on loopback, without Docker:

```sh
oec/demos/check.sh
```

builds `liboec_ffi.a` (release) and the test broker `go-testbroker` of
`oec/sdk/rust`, the demos (the logger also with AddressSanitizer + UBSan),
runs the C unit tests, then `go test -race ./...`: the integration tests of
every demo against the in-process TLS test broker and D7
(`oec-check test`, built from `oec/conformance` and run as a program) at the
demo's level. The tests can also run alone; a missing helper skips them:

```sh
cd oec/demos
OEC_LOGGER=$PWD/logger/target/oec-logger \
OEC_TESTBROKER=<cargo target>/debug/examples/go-testbroker \
OEC_D7=<path of oec-check> go test ./...
```

`OEC_DEMOS_REQUIRE=1` turns a missing helper into a failure (set by
`check.sh`). `OEC_LOGGER_D7_REPORT=<file>` keeps the D7 report of the logger.
