//go:build fips

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestFIPSContainer(t *testing.T) {
	image := os.Getenv("MCP_TEST_FIPS_IMAGE")
	if image == "" {
		t.Skip("set MCP_TEST_FIPS_IMAGE to test a locally built production image")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("mcp-fips-test-%d", time.Now().UnixNano())
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", "--network", "none",
		"--name", name, "-e", "GODEBUG=fips140=only",
		"-e", "VMANOMALY_ENDPOINT=http://127.0.0.1:8490", image)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		cancel()
		_ = cmd.Wait()
		// Killing the Docker client on timeout does not necessarily stop its container.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	decoder := json.NewDecoder(stdout)
	exchange := func(request string, id int) map[string]json.RawMessage {
		t.Helper()
		if _, err := fmt.Fprintln(stdin, request); err != nil {
			t.Fatal(err)
		}
		var response struct {
			ID     int                        `json:"id"`
			Result map[string]json.RawMessage `json:"result"`
			Error  json.RawMessage            `json:"error"`
		}
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("decode response: %v (context: %v)", err, ctx.Err())
		}
		if response.ID != id || len(response.Error) != 0 || response.Result == nil {
			t.Fatalf("unexpected MCP response: %+v", response)
		}
		return response.Result
	}
	initialized := exchange(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"fips-test","version":"1"}}}`, 1)
	var identity struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(initialized["serverInfo"], &identity); err != nil || identity.Name != serverName || identity.Version == "" {
		t.Fatalf("invalid server identity: %s (%v)", initialized["serverInfo"], err)
	}
	if _, err := fmt.Fprintln(stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); err != nil {
		t.Fatal(err)
	}
	result := exchange(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, 2)
	var tools []json.RawMessage
	if err := json.Unmarshal(result["tools"], &tools); err != nil || len(tools) == 0 {
		t.Fatalf("expected nonempty tool list: %s (%v)", result["tools"], err)
	}
}
