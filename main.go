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
var Version = "0.3.0"

func main() {
	exitWithError := func(err error) {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		width = 0
	}
	dbFilePath := appName + "/cache.db"
	if err := run(os.Args, os.Stdin, os.Stdout, width, dbFilePath); err != nil {
		exitWithError(err)
	}
}

func run(args []string, _ io.Reader, stdout io.Writer, width int, dbFilepath string) error {
	fs := pflag.NewFlagSet(args[0], pflag.ExitOnError)
	clearCache := fs.BoolP("clear-cache", "c", false, "clear the local cache before the lookup")
	noSpinner := fs.Bool("no-spinner", false, "do not show spinner")
	logLevel := fs.StringP("log-level", "l", "warn", "set the log level for the current run")
	maxWidth := fs.IntP("max-width", "w", width, "set the maximum width manually. 0 = unlimited")
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
	p, err := xdg.CacheFile(dbFilepath)
	if err != nil {
		return err
	}
	db, err := bolt.Open(p, 0600, nil)
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

	a := NewApp(esiClient, st, stdout)
	a.MaxWidth = *maxWidth
	a.SpinnerDisabled = *noSpinner

	if *clearCache {
		n, err := a.st.Clear()
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "cache cleared (%d objects)\n", n)
	}

	err = a.Run(fs.Args())
	if err != nil {
		return err
	}
	return nil
}
