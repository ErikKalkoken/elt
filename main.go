// elt is a command line tool for looking up Eve Online objects.
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/antihax/goesi"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/spf13/pflag"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	appName           = "elt"
	esiUserAgentEmail = "kalkoken87@gmail.com"
	logLevelDefault   = "info"
	logMaxBackups     = 3
	logMaxSizeMB      = 50
	httpClientTimeout = 30 * time.Second
	sourceURL         = "https://github.com/ErikKalkoken/elt"
)

var ErrNotFound = errors.New("not found")

// Version is overwritten in the CI release process.
var Version = "0.4.0"

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
	dbFilePath, err := xdg.CacheFile(fmt.Sprintf("%s/cache.db", appName))
	if err != nil {
		exitWithError(err)
	}
	logFilePath, err := xdg.StateFile(fmt.Sprintf("%[1]s/%[1]s.log", appName))
	if err != nil {
		exitWithError(err)
	}
	if err := run(os.Args, os.Stdin, os.Stdout, width, dbFilePath, logFilePath); err != nil {
		exitWithError(err)
	}
}

func run(args []string, _ io.Reader, stdout io.Writer, width int, dbFilepath, logFilePath string) error {
	fs := pflag.NewFlagSet(args[0], pflag.ExitOnError)
	clearCache := fs.BoolP("clear-cache", "c", false, "clear the local cache before the lookup")
	noSpinner := fs.Bool("no-spinner", false, "do not show spinner")
	logLevel := fs.StringP("log-level", "l", logLevelDefault, "set the log level for the current run")
	maxWidth := fs.IntP("max-width", "w", width, "set the maximum width manually. 0 = unlimited")
	showVersion := fs.BoolP("version", "v", false, "print the version")
	showFiles := fs.Bool("files", false, "show path to files created by elt")
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
	if *showFiles {
		fmt.Fprintf(stdout, "DB: %s\n", dbFilepath)
		fmt.Fprintf(stdout, "Log: %s\n", logFilePath)
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
	logger := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    logMaxSizeMB,
		MaxBackups: logMaxBackups,
	}
	defer logger.Close()
	log.SetOutput(logger)

	// Setup storage
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
	rhc.ResponseLogHook = logResponse
	rhc.HTTPClient.Timeout = httpClientTimeout
	userAgent := fmt.Sprintf("%s/%s (%s; +%s)", appName, Version, esiUserAgentEmail, sourceURL)
	esiClient := goesi.NewAPIClient(rhc.StandardClient(), userAgent)

	a := NewApp(esiClient, st, stdout)
	a.MaxWidth = *maxWidth
	a.SpinnerDisabled = *noSpinner

	if fs.NArg() == 0 {
		fs.Usage()
		return nil
	}

	if *clearCache {
		n, err := st.Clear()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "cache cleared (%d objects)\n", n)
	}

	err = a.Run(fs.Args())
	if err != nil {
		slog.Error("Run failed", "error", err)
		return err // also need to tell the user about the error
	}
	return nil
}
