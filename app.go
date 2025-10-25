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
	"strconv"
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
	c2 := strings.ReplaceAll(string(c), "_", " ")
	return cases.Title(language.English).String(c2)
}

type Entity struct {
	ID       int32          `json:"id"`
	Category EntityCategory `json:"category"`
	Name     string         `json:"name"`
}

type app struct {
	httpClient *retryablehttp.Client
}

func newApp() app {
	a := app{
		httpClient: retryablehttp.NewClient(),
	}
	a.httpClient.Logger = slog.Default()
	return a
}

func (a app) commandIDs(ctx context.Context, cmd *cli.Command) error {
	if err := setLogLevel(cmd); err != nil {
		return err
	}
	entities, err := a.resolveIDs(cmd.Int32Args("ID"))
	if err != nil {
		return err
	}
	categoryIDs := make(map[EntityCategory][]int32)
	for _, it := range entities {
		categoryIDs[it.Category] = append(categoryIDs[it.Category], it.ID)
	}
	var eveTypes []EveType
	var g errgroup.Group
	for c, ids := range categoryIDs {
		g.Go(func() error {
			switch c {
			case InventoryType:
				x, err := a.resolveEveTypes(ids)
				if err != nil {
					return err
				}
				eveTypes = x
			default:

			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	for _, c := range []EntityCategory{
		Alliance, Character, Constellation, Corporation, Faction, InventoryType, Region, SolarSystem, Station, Invalid,
	} {
		ids, found := categoryIDs[c]
		if !found {
			continue
		}
		fmt.Printf("%s:\n", c.Display())
		switch c {
		case InventoryType:
			if len(eveTypes) > 0 {
				a.printEveTypes(eveTypes)
			}
		default:
			var items []Entity
			for _, id := range ids {
				items = append(items, entities[id])
			}
			a.printEntities(items)
		}
	}
	return nil
}

func (a app) resolveIDs(ids []int32) (map[int32]Entity, error) {
	items, err := a.resolveIDs2(ids)
	if err != nil {
		return nil, err
	}
	items2 := make(map[int32]Entity)
	for _, it := range items {
		items2[it.ID] = it
	}
	return items2, nil
}

func (a app) resolveIDs2(ids []int32) ([]Entity, error) {
	if len(ids) == 0 {
		return []Entity{}, nil
	}
	items, err := a.resolveIDFromAPI(ids)
	if errors.Is(err, errNotFound) {
		n := len(ids)
		if n == 1 {
			return []Entity{{ID: ids[0], Name: "", Category: Invalid}}, nil
		}
		var it1, it2 []Entity
		g := new(errgroup.Group)
		g.Go(func() error {
			items, err := a.resolveIDs2(ids[:n/2])
			if err != nil {
				return err
			}
			it1 = items
			return nil
		})
		g.Go(func() error {
			items, err := a.resolveIDs2(ids[n/2:])
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

func (a app) resolveIDFromAPI(ids []int32) ([]Entity, error) {
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
	items := make([]Entity, 0)
	if err := json.NewDecoder(res.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

type EveType struct {
	Description string `json:"description"`
	GroupID     int32  `json:"group_id"`
	Name        string `json:"name"`
	TypeID      int32  `json:"type_id"`
}

func (a app) resolveEveTypes(ids []int32) ([]EveType, error) {
	items := make([]EveType, len(ids))
	var g errgroup.Group
	for i, id := range ids {
		g.Go(func() error {
			res, err := a.httpClient.Get("https://" + path.Join(esiBaseURL, "universe", "types", strconv.Itoa(int(id))))
			if err != nil {
				return err
			}
			defer res.Body.Close()
			if res.StatusCode == http.StatusNotFound {
				items[i] = EveType{
					TypeID: id,
					Name:   "NOT FOUND",
				}
				return nil
			}
			if res.StatusCode != http.StatusOK {
				return fmt.Errorf("API returned error: %s", res.Status)
			}
			var item EveType
			if err := json.NewDecoder(res.Body).Decode(&item); err != nil {
				return err
			}
			items[i] = item
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return items, nil
}

func (app) printEveTypes(items []EveType) {
	slices.SortFunc(items, func(a, b EveType) int {
		return cmp.Compare(a.TypeID, b.TypeID)
	})
	data := make([][]any, 0)
	for _, item := range items {
		data = append(data, []any{item.TypeID, item.Name, item.Description})
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
	table.Header("ID", "Name", "Description")
	table.Bulk(data)
	table.Render()
}

func (app) printEntities(items []Entity) {
	slices.SortFunc(items, func(a, b Entity) int {
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
