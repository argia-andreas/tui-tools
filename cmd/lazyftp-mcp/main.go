// lazyftp-mcp serves only MCP over stdio. Diagnostics never go to stdout.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	lazymcp "lazyftp/internal/mcp"
	"lazyftp/internal/storage"
)

func run() error {
	policyFile := flag.String("policy", "", "operator-owned JSON permission policy (omitted denies everything)")
	config := flag.String("rclone-config", "", "absolute operator-owned rclone config; required with grants")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	var policy lazymcp.Policy
	if *policyFile != "" {
		f, err := os.Open(*policyFile)
		if err != nil {
			return fmt.Errorf("cannot open MCP policy")
		}
		defer f.Close()
		policy, err = lazymcp.ReadPolicy(f)
		if err != nil {
			return err
		}
	}
	if len(policy.Grants) != 0 {
		if err := policy.ProtectConfig(*config); err != nil {
			return err
		}
	}
	server, err := lazymcp.New(policy, storage.NewRclone(*config))
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		// Transport errors can contain malformed client input, including secrets.
		return fmt.Errorf("stdio connection failed")
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazyftp-mcp:", err)
		os.Exit(1)
	}
}
