//go:build fips

package main

import (
	"crypto/fips140"
	"fmt"
)

// Match the frozen module selected in Makefile and .goreleaser.yaml. A build tag
// alone or GODEBUG=fips140=on with the toolchain's current module is insufficient.
const requiredFIPSModule = "v1.0.0"

func checkCryptoMode() error {
	if !fips140.Enabled() {
		return fmt.Errorf("this FIPS build requires FIPS mode; remove GODEBUG=fips140=off")
	}
	if version := fips140.Version(); version != requiredFIPSModule {
		return fmt.Errorf("this FIPS build requires Go Cryptographic Module %s, got %s", requiredFIPSModule, version)
	}
	return nil
}

func cryptoBuildInfo() string {
	return fmt.Sprintf(" (FIPS module: %s, enabled: %t)", fips140.Version(), fips140.Enabled())
}
