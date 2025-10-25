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
	"os"
	"path"
	"slices"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type EntityCategory string

// Supported categories of EveEntity
const (
	Undefined     EntityCategory = ""
	Alliance      EntityCategory = "alliance"
	Character     EntityCategory = "character"
	Constellation EntityCategory = "constellation"
	Corporation   EntityCategory = "corporation"
	Faction       EntityCategory = "faction"
	InventoryType EntityCategory = "inventory_type"
	Region        EntityCategory = "region"
	SolarSystem   EntityCategory = "solar_system"
	Station       EntityCategory = "station"
	Invalid       EntityCategory = "invalid"
)

func (c EntityCategory) Display() string {
	if c == Invalid {
		return "INVALID"
	}
	c2 := strings.ReplaceAll(string(c), "_", " ")
	return cases.Title(language.English).String(c2)
}

type EveEntity struct {
	ID       int32          `json:"id"`
	Category EntityCategory `json:"category"`
	Name     string         `json:"name"`
}

type App struct {
	httpClient *retryablehttp.Client
}

func NewApp() App {
	a := App{
		httpClient: retryablehttp.NewClient(),
	}
	a.httpClient.Logger = slog.Default()
	return a
}

func (a App) CommandIDs(ctx context.Context, cmd *cli.Command) error {
	if err := setLogLevel(cmd); err != nil {
		return err
	}
	items, err := a.resolveIDs(cmd.Int32Args("ID"))
	if err != nil {
		return err
	}
	a.printEveEntities(items)
	return nil
}

func (a App) resolveIDs(ids []int32) ([]EveEntity, error) {
	if len(ids) == 0 {
		return []EveEntity{}, nil
	}
	items, err := a.resolveIDFromAPI(ids)
	if errors.Is(err, errNotFound) {
		n := len(ids)
		if n == 1 {
			return []EveEntity{{ID: ids[0], Name: "", Category: Invalid}}, nil
		}
		var it1, it2 []EveEntity
		g := new(errgroup.Group)
		g.Go(func() error {
			items, err := a.resolveIDs(ids[:n/2])
			if err != nil {
				return err
			}
			it1 = items
			return nil
		})
		g.Go(func() error {
			items, err := a.resolveIDs(ids[n/2:])
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

func (a App) resolveIDFromAPI(ids []int32) ([]EveEntity, error) {
	body, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	r, err := retryablehttp.NewRequest("POST", "https://"+path.Join(esiBaseURL, "universe", "names"), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	r.Header.Add("Content-Type", "application/json")
	res, err := a.httpClient.Do(r)
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
	items := make([]EveEntity, 0)
	if err := json.NewDecoder(res.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

func (App) printEveEntities(items []EveEntity) {
	slices.SortFunc(items, func(a, b EveEntity) int {
		return cmp.Compare(a.ID, b.ID)
	})
	data := make([][]any, 0)
	for _, item := range items {
		data = append(data, []any{item.ID, item.Name, item.Category.Display()})
	}
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Settings: tw.Settings{Separators: tw.Separators{BetweenRows: tw.On}},
		})),
		tablewriter.WithConfig(tablewriter.Config{
			MaxWidth: 200,
			Row: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoWrap: tw.WrapNormal},
				Alignment:  tw.CellAlignment{Global: tw.AlignLeft}, // Left-align rows
			},
		}),
	)
	table.Header("ID", "Name", "Category")
	table.Bulk(data)
	table.Render()
}
