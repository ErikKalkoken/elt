// elt is a command line tool for looking up Eve Online objects.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/adrg/xdg"
	"github.com/antihax/goesi"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/urfave/cli/v3"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/term"
)

const (
	appName        = "elt"
	userAgentEmail = "kalkoken87@gmail.com"
	sourceURL      = "https://github.com/ErikKalkoken/elt"
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

const description = `This command looks up EVE Online objects from the game server and prints them in the terminal.

You can pass in a mix of EVE IDs and names. Please use quotes for names with multiple words.

EVE objects of the following categories are supported:
Agents, Alliances, Characters, Constellations, Corporations, Factions, Regions, Stations, Solar Systems, Types

Example:

elt 30000142 "Erik Kalkoken"

For more information please see this website: ` + sourceURL

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

	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return err
	}
	app := NewApp(esiClient, st, stdout, width)

	cmd := &cli.Command{
		Usage:       "A command line tool for looking up Eve Online objects.",
		ArgsUsage:   "value1 [value2 value3 ...]",
		Description: description,
		Version:     Version,
		Authors:     []any{"Erik Kalkoken"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Value:   "info",
				Usage:   "log level for this sessions",
			},
			&cli.BoolFlag{
				Name:  "clear-cache",
				Usage: "Clears the local cache before the lookup",
			},
		},
		Action: app.Run,
	}
	if err := cmd.Run(context.Background(), args); err != nil {
		return err
	}
	return nil
}
