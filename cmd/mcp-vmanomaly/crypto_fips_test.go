//go:build fips

package main

import (
	"bytes"
	"crypto/fips140"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFIPSBuildIdentity(t *testing.T) {
	if !fips140.Enabled() || fips140.Version() != requiredFIPSModule {
		t.Fatalf("unexpected crypto module: enabled=%v version=%s", fips140.Enabled(), fips140.Version())
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("version failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "FIPS module: "+requiredFIPSModule+", enabled: true") {
		t.Fatalf("missing crypto build identity: %s", stdout.String())
	}
}

func TestFIPSRejectsDisabledMode(t *testing.T) {
	if os.Getenv("MCP_TEST_FIPS_DISABLED_CHILD") == "1" {
		os.Exit(run([]string{"--version"}, os.Stdout, os.Stderr))
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-test.run=^TestFIPSRejectsDisabledMode$")
	cmd.Env = append(os.Environ(), "GODEBUG=fips140=off", "MCP_TEST_FIPS_DISABLED_CHILD=1")
	output, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(output, []byte("this FIPS build requires FIPS mode")) {
		t.Fatalf("disabled mode was not rejected by the startup guard: %v: %s", err, output)
	}
}
