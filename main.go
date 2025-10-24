// eveid is a command line tool for resolved Eve Online IDs to names and categories.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

const (
	esiBaseURL = "esi.evetech.net"
)

var errNotFound = errors.New("not found")

func main() {
	cmd := &cli.Command{
		Usage: "A command line tool for querying information about Eve Online objects.\nThe results are printed to stdout in JSON format.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Value:   "info",
				Usage:   "log level for this sessions",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "ids",
				Usage:  "resolve entity IDs to names and categories",
				Action: ids,
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "sort-name", Usage: "sort results by name"},
				},
				Arguments: []cli.Argument{
					&cli.Int32Args{
						Name: "ID",
						Min:  1,
						Max:  -1,
					},
				},
			},
			{
				Name:   "types",
				Usage:  "fetch info about a type",
				Action: types,
				Arguments: []cli.Argument{
					&cli.Int32Args{
						Name: "ID",
						Min:  1,
						Max:  -1,
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Println("ERROR: " + err.Error())
		os.Exit(1)
	}
}

func setLogLevel(cmd *cli.Command) error {
	m := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	l, ok := m[strings.ToLower(cmd.String("log-level"))]
	if !ok {
		msg := fmt.Sprintf("valid log levels are %s", strings.Join(slices.Collect(maps.Keys(m)), ", "))
		return cli.Exit(msg, 1)
	}
	slog.SetLogLoggerLevel(l)
	return nil
}
