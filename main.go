// everef is a command line tool for getting information about Eve Online objects.
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

	"github.com/hashicorp/go-retryablehttp"
	"github.com/urfave/cli/v3"
)

const (
	esiBaseURL = "esi.evetech.net"
)

var errNotFound = errors.New("not found")

func main() {
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = slog.Default()
	app := NewApp(httpClient)
	cmd := &cli.Command{
		Usage:   "A command line tool for getting information about Eve Online objects.",
		Version: "0.1.0",
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
				Usage:  "resolves entities from IDs",
				Action: app.ResolveIDs,
				Arguments: []cli.Argument{
					&cli.Int32Args{
						Name: "ID",
						Min:  1,
						Max:  -1,
					},
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name: "sort-category",
					},
					&cli.BoolFlag{
						Name: "sort-id",
					},
					&cli.BoolFlag{
						Name: "sort-name",
					},
				},
			},
			{
				Name:   "names",
				Usage:  "resolve entities from names",
				Action: app.ResolveNames,
				Arguments: []cli.Argument{
					&cli.StringArgs{
						Name: "Name",
						Min:  1,
						Max:  -1,
					},
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name: "sort-category",
					},
					&cli.BoolFlag{
						Name: "sort-id",
					},
					&cli.BoolFlag{
						Name: "sort-name",
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
