package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
)

type Item struct {
	ID       int32  `json:"id"`
	Category string `json:"category"`
	Name     string `json:"name"`
}

func ids(ctx context.Context, cmd *cli.Command) error {
	if err := setLogLevel(cmd); err != nil {
		return err
	}
	items, err := resolveIDs2(cmd.Int32Args("ID"))
	if err != nil {
		return err
	}
	if cmd.Bool("sort-name") {
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
		return err
	}
	fmt.Println(string(out))
	return nil
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
	r, err := retryablehttp.NewRequest("POST", "https://"+path.Join(esiBaseURL, "universe", "names"), bytes.NewBuffer(body))
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

// func parseInput(s string) ([]int32, error) {
// 	p := strings.Split(s, ",")
// 	if len(p) == 0 {
// 		return nil, fmt.Errorf("no input")
// 	}
// 	ids := make([]int32, 0)
// 	for _, x := range p {
// 		id, err := strconv.Atoi(x)
// 		if err != nil {
// 			return nil, err
// 		}
// 		id2 := int32(id)
// 		if int(id2) != id {
// 			return nil, fmt.Errorf("number exceeding int32: %d", id)
// 		}
// 		ids = append(ids, id2)
// 	}
// 	return ids, nil
// }
