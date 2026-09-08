package cli

import (
	"context"
	"errors"
	"os"

	"github.com/urfave/cli/v3"

	"olexsmir.xyz/clerk/internal/lsp"
)

func (c *Cli) lspAction(ctx context.Context, cmd *cli.Command) error {
	configPath, err := findConfigFilePath(cmd)
	if err != nil {
		return err
	}
	server, err := lsp.NewServer(c.version, configPath)
	if err != nil {
		return err
	}
	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
		if lee, ok := errors.AsType[*lsp.ExitError](err); ok {
			os.Exit(lee.Code)
		}
		return err
	}
	return nil
}
