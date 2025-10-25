package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	Agent         EntityCategory = "agent"
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

func NewApp(httpClient *retryablehttp.Client) App {
	a := App{
		httpClient: httpClient,
	}
	return a
}

func (a App) ResolveIDs(ctx context.Context, cmd *cli.Command) error {
	if err := setLogLevel(cmd); err != nil {
		return err
	}
	entities, err := a.resolveIDs(cmd.Int32Args("ID"))
	if err != nil {
		return err
	}
	sortEntities(cmd.Bool("sort-category"), cmd.Bool("sort-id"), cmd.Bool("sort-name"), entities)
	a.printEveEntities(entities)
	return nil
}

func (a App) resolveIDs(ids []int32) ([]EveEntity, error) {
	if len(ids) == 0 {
		return []EveEntity{}, nil
	}
	entities, err := a.resolveIDFromAPI(ids)
	if errors.Is(err, errNotFound) {
		n := len(ids)
		if n == 1 {
			return []EveEntity{{ID: ids[0], Name: "", Category: Invalid}}, nil
		}
		var it1, it2 []EveEntity
		g := new(errgroup.Group)
		g.Go(func() error {
			entities, err := a.resolveIDs(ids[:n/2])
			if err != nil {
				return err
			}
			it1 = entities
			return nil
		})
		g.Go(func() error {
			entities, err := a.resolveIDs(ids[n/2:])
			if err != nil {
				return err
			}
			it2 = entities
			return nil
		})
		if err := g.Wait(); err != nil {
			return nil, err
		}
		entities = slices.Concat(it1, it2)
		return entities, nil
	}
	if err != nil {
		return nil, err
	}
	m := make(map[int32]EveEntity)
	for _, e := range entities {
		m[e.ID] = e
	}
	entities2 := make([]EveEntity, 0)
	for _, id := range ids {
		entities2 = append(entities2, m[id])
	}
	return entities2, nil
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
	entities := make([]EveEntity, 0)
	if err := json.NewDecoder(res.Body).Decode(&entities); err != nil {
		return nil, err
	}
	return entities, nil
}

func (a App) ResolveNames(ctx context.Context, cmd *cli.Command) error {
	if err := setLogLevel(cmd); err != nil {
		return err
	}
	entities, err := a.resolveNames(cmd.StringArgs("Name"))
	if err != nil {
		return err
	}
	sortEntities(cmd.Bool("sort-category"), cmd.Bool("sort-id"), cmd.Bool("sort-name"), entities)
	a.printEveEntities(entities)
	return nil
}

func (a App) resolveNames(names []string) ([]EveEntity, error) {
	body, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}
	r, err := retryablehttp.NewRequest("POST", "https://"+path.Join(esiBaseURL, "universe", "ids"), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	r.Header.Add("Content-Type", "application/json")
	res, err := a.httpClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error: %s", res.Status)
	}
	var data map[string][]EveEntity
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, err
	}
	found := make(map[string]bool)
	entities := make([]EveEntity, 0)
	for category, items := range data {
		var ec EntityCategory
		switch category {
		case "agents":
			ec = Agent
		case "alliances":
			ec = Alliance
		case "characters":
			ec = Character
		case "corporations":
			ec = Corporation
		case "constellations":
			ec = Constellation
		case "factions":
			ec = Faction
		case "inventory_types":
			ec = InventoryType
		case "regions":
			ec = Region
		case "stations":
			ec = Station
		case "systems":
			ec = SolarSystem
		}
		for _, it := range items {
			it.Category = ec
			entities = append(entities, it)
			found[it.Name] = true
		}
	}
	for _, n := range names {
		if found[n] {
			continue
		}
		entities = append(entities, EveEntity{Name: n, Category: Invalid})
	}
	return entities, nil
}

func sortEntities(sortCategory, sortID, sortName bool, entities []EveEntity) {
	if !sortID && !sortName && !sortCategory {
		return
	}
	slices.SortFunc(entities, func(a, b EveEntity) int {
		if sortCategory && a.Category != b.Category {
			return strings.Compare(string(a.Category), string(b.Category))
		}
		if sortName && a.Name != b.Name {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func (App) printEveEntities(entities []EveEntity) {
	data := make([][]any, 0)
	for _, item := range entities {
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
