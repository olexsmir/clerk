package cli

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"
	"olexsmir.xyz/clerk/internal/lsp"
)

func (c *Cli) lspAction(ctx context.Context, cmd *cli.Command) error {
	server := lsp.NewServer(c.version)
	return server.Run(ctx, os.Stdin, os.Stdout)
}
