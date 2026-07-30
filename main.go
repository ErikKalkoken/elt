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

	"github.com/ErikKalkoken/eveauth"
	"github.com/adrg/xdg"
	"github.com/fnt-eve/goesi-openapi"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/spf13/pflag"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	appName           = "elt"
	esiUserAgentEmail = "kalkoken87@gmail.com"
	httpClientTimeout = 30 * time.Second
	logLevelDefault   = "info"
	logMaxBackups     = 3
	logMaxSizeMB      = 50
	maxResultsDefault = 25
	sourceURL         = "https://github.com/ErikKalkoken/elt"
	ssoPort           = 30333
	ssoClientID       = "0b2d75d9d16646ddb86b97824d405d52"
)

var ErrNotFound = errors.New("not found")

// Version is overwritten in the CI release process.
var Version = "0.7.0"

var logLevelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

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

	kindValues := []string{}
	for _, x := range []EveEntityCategory{
		CategoryAgent,
		CategoryAlliance,
		CategoryCharacter,
		CategoryConstellation,
		CategoryCorporation,
		CategoryFaction,
		CategoryInventoryType,
		CategoryRegion,
		CategorySolarSystem,
		CategoryStation,
	} {
		kindValues = append(kindValues, string(x))
	}
	kind := newEnumValue(kindValues, "")
	fs.VarP(kind, "kind", "k", kind.FormatDescription("kind of the provided values"))

	logLevel := newEnumValue(slices.Sorted(maps.Keys(logLevelMap)), logLevelDefault)
	fs.VarP(logLevel, "log-level", "l", logLevel.FormatDescription("set the log level for the current run"))

	authorize := fs.Bool("authorize", false, "authorize elt for using search (desktops only)")
	clearCache := fs.Bool("clear-cache", false, "clear the local cache before the lookup")
	maxWidth := fs.IntP("max-width", "w", width, "set the maximum width manually. 0 = unlimited")
	noSpinner := fs.Bool("no-spinner", false, "do not show spinner")
	search := fs.StringP("search", "s", "", "perform search instead of lookup (requires authorization)")
	showFiles := fs.Bool("files", false, "show path to files created by elt")
	showVersion := fs.BoolP("version", "v", false, "print the version")
	maxResults := fs.Int("max-results", maxResultsDefault, "set the maximum number of returned results. 0 = unlimited")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
  elt [options] [value [value ...]]

Description:
  This command looks up EVE Online objects from the game server and prints them in the terminal.
  For more information please see this website: `+sourceURL+`

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
Examples:
  elt 30000142
  elt "Erik Kalkoken" 603
  elt -s merlin`)
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintf(stdout, "Version %s\n", Version)
		return nil
	}
	if *showFiles {
		fmt.Fprintf(stdout, "Cache: %s\n", dbFilepath)
		fmt.Fprintf(stdout, "Log: %s\n", logFilePath)
		return nil
	}
	// Set log level
	l, ok := logLevelMap[strings.ToLower(logLevel.value)]
	if !ok {
		return fmt.Errorf("invalid log level")
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

	if *clearCache {
		n, err := st.ClearCached()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "cache cleared (%d objects)\n", n)
		return nil
	}

	// retryablehttp
	rhc := retryablehttp.NewClient()
	rhc.Logger = slog.Default()
	rhc.ResponseLogHook = logResponse
	rhc.HTTPClient.Timeout = httpClientTimeout

	// eveauth
	authClient, err := eveauth.NewClient(eveauth.Config{
		ApplicationName: appName,
		ClientID:        ssoClientID,
		Port:            ssoPort,
		Logger:          slog.Default(),
	})
	if err != nil {
		return err
	}

	// goesi
	userAgent := fmt.Sprintf("%s/%s (%s; +%s)", appName, Version, esiUserAgentEmail, sourceURL)
	esiClient := goesi.NewESIClientWithOptions(rhc.StandardClient(), goesi.ClientOptions{
		UserAgent: userAgent,
	})

	a := NewApp(authClient, esiClient, st, stdout)
	a.MaxWidth = *maxWidth
	a.MaxResults = *maxResults
	a.SpinnerDisabled = *noSpinner
	a.EntityCategory = EveEntityCategory(kind.value)

	// Authorize app
	if *authorize {
		err := a.Authorize()
		if err != nil {
			return err
		}
		return nil
	}

	if *search != "" {
		err = a.Search(*search)
		if err != nil {
			slog.Error("Search failed", "error", err)
			return err // also need to tell the user about the error
		}
		return nil
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return nil
	}

	err = a.Lookup(fs.Args())
	if err != nil {
		slog.Error("Lookup failed", "error", err)
		return err // also need to tell the user about the error
	}
	return nil
}
