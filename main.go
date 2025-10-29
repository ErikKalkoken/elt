// everef is a command line tool for getting information about Eve Online objects.
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
		Usage:   "A command line tool for looking up Eve Online objects.",
		Version: Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Value:   "info",
				Usage:   "log level for this sessions",
			},
			&cli.BoolFlag{Name: "clear-cache"},
		},
		Action: app.Run,
		Arguments: []cli.Argument{
			&cli.StringArgs{
				Name:      "Value",
				UsageText: "An ID or a name of an EVE online object.",
				Min:       1,
				Max:       -1,
			},
		},
	}
	if err := cmd.Run(context.Background(), args); err != nil {
		return err
	}
	return nil
}
