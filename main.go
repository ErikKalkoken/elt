// everef is a command line tool for getting information about Eve Online objects.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/adrg/xdg"
	"github.com/antihax/goesi"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/urfave/cli/v3"
	bolt "go.etcd.io/bbolt"
)

const (
	appName        = "everef"
	userAgentEmail = "kalkoken87@gmail.com"
	sourceURL      = "https://github.com/ErikKalkoken/everef"
)

var ErrNotFound = errors.New("not found")

// Version is overwritten in the CI release process.
var Version = "0.1.0"

func main() {
	err := run(os.Args, os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

func run(args []string, _ io.Reader, stdout io.Writer) error {
	dbFilepath, err := xdg.CacheFile(appName + "/cache.db")
	if err != nil {
		return err
	}
	db, err := bolt.Open(dbFilepath, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	st := NewStorage(db)
	if err := st.Init(); err != nil {
		return err
	}

	rhc := retryablehttp.NewClient()
	rhc.Logger = slog.Default()
	userAgent := fmt.Sprintf("%s/%s (%s; +%s)", appName, Version, userAgentEmail, sourceURL)
	esiClient := goesi.NewAPIClient(rhc.StandardClient(), userAgent)

	app := NewApp(esiClient, st, stdout)

	cmd := &cli.Command{
		Usage:   "A command line tool for getting information about Eve Online objects.",
		Version: Version,
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
			},
			{
				Name:  "system",
				Usage: "system utilities",
				Commands: []*cli.Command{
					{
						Name:   "dump-cache",
						Usage:  "dump cached objects",
						Action: app.DumpCache,
					},
					{
						Name:   "clear-cache",
						Usage:  "clear all cached objects",
						Action: app.ClearCache,
					},
					{
						Name:  "files",
						Usage: "list files in use",
						Action: func(ctx context.Context, c *cli.Command) error {
							fmt.Printf("DB: %s\n", dbFilepath)
							return nil
						},
					},
				},
			},
		},
	}
	if err := cmd.Run(context.Background(), args); err != nil {
		return err
	}
	return nil
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
