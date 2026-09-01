package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"olexsmir.xyz/clerk/internal/lsp"
)

func (c *Cli) lspAction(ctx context.Context, cmd *cli.Command) error {
	sets, warns, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	for i := range warns {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warns[i])
	}

	server, err := lsp.NewServer(c.version, sets)
	if err != nil {
		return err
	}
	return server.Run(ctx, os.Stdin, os.Stdout)
}
