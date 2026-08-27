package cli

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"

	"olexsmir.xyz/clerk/internal/lsp"
)

func (c *Cli) lspAction(ctx context.Context, cmd *cli.Command) error {
	configPath, err := resolveConfigPath(cmd)
	if err != nil {
		return err
	}
	server, err := lsp.NewServer(c.version, configPath)
	if err != nil {
		return err
	}
	return server.Run(ctx, os.Stdin, os.Stdout)
}
