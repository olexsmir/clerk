package cli

import (
	"context"

	"github.com/urfave/cli/v3"
)

type Cli struct {
	version string
}

func New(version string) *Cli {
	return &Cli{
		version: version,
	}
}

func (c *Cli) Run(ctx context.Context, args []string) error {
	cmd := &cli.Command{
		Name:                  "clerk",
		Usage:                 "missing pta tooling",
		Version:               c.version,
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			{
				Name:   "lsp",
				Usage:  "Run LSP server, to be used by editors",
				Action: c.lspAction,
			},
			{
				Name:                   "format",
				Usage:                  "Format journal files",
				Action:                 c.formatAction,
				UseShortOptionHandling: true,
				Arguments: []cli.Argument{&cli.StringArgs{
					Name:      "journals",
					UsageText: "(path to journal files/directories (stdin if empty))",
					Min:       0,
					Max:       -1,
				}},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "write",
						Aliases: []string{"w"},
						Usage:   "write result back to file instead of stdout",
					},
					&cli.BoolFlag{
						Name:    "check",
						Aliases: []string{"c"},
						Usage:   "exit code 0 if already formatted, 1 otherwise",
					},
					&cli.BoolFlag{
						Name:    "diff",
						Aliases: []string{"d"},
						Usage:   "display diffs instead of rewriting files",
					},
					&cli.BoolFlag{
						Name:    "list",
						Aliases: []string{"l"},
						Usage:   "list files whose formatting differs",
					},
				},
			},
			{
				Name:   "lint",
				Usage:  "Lint journal files",
				Action: c.lintAction,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Usage:   "output format: text, json",
						Value:   "text",
					},
					&cli.StringFlag{
						Name:  "path-style",
						Usage: "file path style: basename, relative, absolute",
						Value: "relative",
					},
				},
				Arguments: []cli.Argument{&cli.StringArgs{
					Name:      "journals",
					UsageText: "(path to journal files/directories (stdin if empty))",
					Min:       0,
					Max:       -1,
				}},
			},
		},
	}
	return cmd.Run(ctx, args)
}
