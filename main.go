// elt is a command line tool for looking up Eve Online objects.
package main

import (
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
	"github.com/spf13/pflag"
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
var Version = "0.2.1"

func main() {
	err := run(os.Args, os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

func run(args []string, _ io.Reader, stdout io.Writer) error {
	fs := pflag.NewFlagSet(args[0], pflag.ExitOnError)
	clearCache := fs.BoolP("clear-cache", "c", false, "clear the local cache before the lookup")
	logLevel := fs.StringP("log-level", "l", "info", "set the log level for the current run")
	showVersion := fs.BoolP("version", "v", false, "print the version")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
  elt [options] value [value ...]

Description:
  This command looks up EVE Online objects from the game server and prints them in the terminal.
  For more information please see this website: `+sourceURL+`

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
Examples:
  elt 30000142
  elt "Erik Kalkoken" 603`)
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintf(stdout, "Version %s\n", Version)
		return nil
	}
	// Set log level
	m := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	l, ok := m[strings.ToLower(*logLevel)]
	if !ok {
		return fmt.Errorf("valid log levels are: %s", strings.Join(slices.Collect(maps.Keys(m)), ", "))
	}
	slog.SetLogLoggerLevel(l)
	if fs.NArg() == 0 {
		fs.Usage()
		return nil
	}

	// Setup storage
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

	// Setup clients
	rhc := retryablehttp.NewClient()
	rhc.Logger = slog.Default()
	userAgent := fmt.Sprintf("%s/%s (%s; +%s)", appName, Version, userAgentEmail, sourceURL)
	esiClient := goesi.NewAPIClient(rhc.StandardClient(), userAgent)

	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return err
	}
	app := NewApp(esiClient, st, stdout, width)
	err = app.Run(fs.Args(), *clearCache)
	if err != nil {
		return err
	}
	// cmd := &cli.Command{
	// 	Usage:       "A command line tool for looking up Eve Online objects.",
	// 	ArgsUsage:   "value1 [value2 value3 ...]",
	// 	Description: description,
	// 	Version:     Version,
	// 	Authors:     []any{"Erik Kalkoken"},
	// 	Flags: []cli.Flag{
	// 		&cli.StringFlag{
	// 			Name:    "log-level",
	// 			Aliases: []string{"l"},
	// 			Value:   "info",
	// 			Usage:   "log level for this sessions",
	// 		},
	// 		&cli.BoolFlag{
	// 			Name:  "clear-cache",
	// 			Usage: "Clears the local cache before the lookup",
	// 		},
	// 	},
	// 	Action: app.Run,
	// }

	return nil
}
