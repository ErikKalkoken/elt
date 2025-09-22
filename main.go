// eveid is a command line tool for resolved Eve Online IDs to names and categories.
package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
	"golang.org/x/sync/errgroup"
)

var errNotFound = errors.New("not found")

type Item struct {
	ID       int32  `json:"id"`
	Category string `json:"category"`
	Name     string `json:"name"`
}

func main() {
	exitWithError := func(err error) {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	sortNameFlag := flag.Bool("sort-name", false, "Sort results by name")
	logLevelFlag := flag.String("log-level", "", "Set log level for this session")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "eveid is a command line tool for resolving Eve Online IDs to names and categories. The results are printed to stdout in JSON format.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: eveid [flags] ids\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// set manual log level for this session if requested
	if v := *logLevelFlag; v != "" {
		m := map[string]slog.Level{
			"debug": slog.LevelDebug,
			"info":  slog.LevelInfo,
			"warn":  slog.LevelWarn,
			"error": slog.LevelError,
		}
		l, ok := m[strings.ToLower(v)]
		if !ok {
			fmt.Println("valid log levels are: ", strings.Join(slices.Collect(maps.Keys(m)), ", "))
			os.Exit(1)
		}
		slog.SetLogLoggerLevel(l)
	}

	args := flag.Args()
	if len(args) != 1 {
		exitWithError(fmt.Errorf("no IDs provided"))
	}

	ids, err := parseInput(args[0])
	if err != nil {
		exitWithError(err)
	}

	items, err := resolveIDs2(ids)
	if err != nil {
		exitWithError(err)
	}
	if *sortNameFlag {
		slices.SortFunc(items, func(a, b Item) int {
			return strings.Compare(a.Name, b.Name)
		})
	} else {
		slices.SortFunc(items, func(a, b Item) int {
			return cmp.Compare(a.ID, b.ID)
		})
	}
	out, err := json.MarshalIndent(items, "", "    ")
	if err != nil {
		exitWithError(err)
	}
	fmt.Println(string(out))
}

func resolveIDs2(ids []int32) ([]Item, error) {
	if len(ids) == 0 {
		return []Item{}, nil
	}
	items, err := resolveIDs(ids)
	if errors.Is(err, errNotFound) {
		n := len(ids)
		if n == 1 {
			return []Item{{ID: ids[0], Name: "", Category: "invalid"}}, nil
		}
		var it1, it2 []Item
		g := new(errgroup.Group)
		g.Go(func() error {
			items, err := resolveIDs2(ids[:n/2])
			if err != nil {
				return err
			}
			it1 = items
			return nil
		})
		g.Go(func() error {
			items, err := resolveIDs2(ids[n/2:])
			if err != nil {
				return err
			}
			it2 = items
			return nil
		})
		if err := g.Wait(); err != nil {
			return nil, err
		}
		items = slices.Concat(it1, it2)
		return items, nil
	}
	if err != nil {
		return nil, err
	}
	return items, nil
}

func resolveIDs(ids []int32) ([]Item, error) {
	body, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	r, err := retryablehttp.NewRequest("POST", "https://esi.evetech.net/universe/names", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	r.Header.Add("Content-Type", "application/json")

	client := retryablehttp.NewClient()
	client.Logger = slog.Default()
	res, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error: %s", res.Status)
	}
	items := make([]Item, 0)
	if err := json.NewDecoder(res.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

func parseInput(s string) ([]int32, error) {
	p := strings.Split(s, ",")
	if len(p) == 0 {
		return nil, fmt.Errorf("no input")
	}
	ids := make([]int32, 0)
	for _, x := range p {
		id, err := strconv.Atoi(x)
		if err != nil {
			return nil, err
		}
		id2 := int32(id)
		if int(id2) != id {
			return nil, fmt.Errorf("number exceeding int32: %d", id)
		}
		ids = append(ids, id2)
	}
	return ids, nil
}
