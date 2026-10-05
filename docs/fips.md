# FIPS builds

The opt-in Linux amd64/arm64 variant selects Go Cryptographic Module v1.0.0 at build time with `GOFIPS140=v1.0.0`. This module is covered by [CMVP certificate #5247](https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/5247). Using the module does not by itself certify the whole application or deployment; check the certificate’s operating environments and security policy against the intended deployment.

## Build and verify

Use the Go toolchain specified in `go.mod`:

```sh
make build-fips
./bin/mcp-vmanomaly-fips --version
make test-fips
make docker-build-fips
MCP_TEST_FIPS_IMAGE=mcp-vmanomaly:fips GOFIPS140=v1.0.0 go test -tags=fips -run TestFIPSContainer ./cmd/mcp-vmanomaly
```

The `fips` build tag enables a startup guard. It rejects a disabled FIPS mode or a different crypto-module version before starting the service. Building with the tag alone is insufficient: use the Make target or the matching GoReleaser build. `--version` reports the selected module and whether it is enabled. Ordinary builds retain their existing behavior.

The FIPS image packages the static binary and CA certificates into scratch and runs as UID/GID 1000. It contains no shell or package manager. GoReleaser defines separate `-fips` image tags and `_fips` release archives for Linux amd64/arm64; these artifacts become available only after a release containing this change. Existing standard artifacts and their names are preserved.

## Crypto and transport scope

Outbound HTTPS uses Go’s `crypto/tls` and certificate verification. FIPS mode restricts TLS algorithms and versions to the supported approved set. The search index dependency is upgraded to Vellum 1.2.0, replacing its MD5 initialization path with SHA-256. CA trust, endpoint credentials, and network access policies still need normal deployment configuration.

The build does not add TLS termination to inbound MCP HTTP/SSE transports. Use the existing authenticated TLS proxy deployment model where remote transport is required. Any proxy’s cryptographic module and operating environment need their own assessment. Stdio is a local process transport, not an encrypted network channel.

CI runs the full Go suite with the pinned module and again with `GODEBUG=fips140=only`. It checks HTTPS with TLS 1.2/1.3, rejection of TLS 1.1 and untrusted certificates, startup rejection when FIPS mode is disabled, and an MCP initialize/tools-list exchange in the final container on native amd64 and arm64 runners. The strict `only` setting is a diagnostic test mode, not the production default. Tests cover these exercised paths and do not constitute a compliance certification.

When changing the crypto-module pin, update `Makefile`, `.goreleaser.yaml`, the startup guard, CI and this document together; verify the new module’s validation status and deployment requirements. See [Go’s FIPS documentation](https://go.dev/doc/security/fips140) for module selection and runtime behavior.
