//go:build fips

package main

import (
	"crypto/fips140"
	_ "embed"
	"fmt"
	"strings"
)

// Make, GoReleaser and CI read this same pin. Embed it so the production image
// needs no configuration file and the guard can still reject a mismatched build.
//
//go:embed fips_module.txt
var fipsModulePin string

var requiredFIPSModule = strings.TrimSpace(fipsModulePin)

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
