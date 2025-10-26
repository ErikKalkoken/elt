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

	"github.com/adrg/xdg"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/urfave/cli/v3"
	bolt "go.etcd.io/bbolt"
)

const (
	esiBaseURL = "esi.evetech.net"
)

var ErrNotFound = errors.New("not found")

func main() {
	exitWithError := func(err error) {
		fmt.Println("ERROR: " + err.Error())
		os.Exit(1)
	}
	p, err := xdg.CacheFile("everef/cache.db")
	if err != nil {
		exitWithError(err)
	}
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		exitWithError(err)
	}
	defer db.Close()
	st := NewStorage(db)
	if err := st.Init(); err != nil {
		exitWithError(err)
	}

	httpClient := retryablehttp.NewClient()
	httpClient.Logger = slog.Default()

	app := NewApp(httpClient, st)

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
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if err := setLogLevel(cmd); err != nil {
				return ctx, err
			}
			return ctx, nil
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
			{
				Name:  "cache",
				Usage: "manage cached entities",
				Commands: []*cli.Command{
					{
						Name:   "list",
						Usage:  "list objects",
						Action: app.ListCache,
					},
					{
						Name:   "clear",
						Usage:  "clear objects",
						Action: app.ClearCache,
					},
				},
			},
		},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		exitWithError(err)
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
