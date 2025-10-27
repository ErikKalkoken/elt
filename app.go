package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/antihax/goesi"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type EveEntityCategory string

// Supported categories of EveEntity
const (
	Undefined     EveEntityCategory = ""
	Agent         EveEntityCategory = "agent"
	Alliance      EveEntityCategory = "alliance"
	Character     EveEntityCategory = "character"
	Constellation EveEntityCategory = "constellation"
	Corporation   EveEntityCategory = "corporation"
	Faction       EveEntityCategory = "faction"
	InventoryType EveEntityCategory = "inventory_type"
	Region        EveEntityCategory = "region"
	SolarSystem   EveEntityCategory = "solar_system"
	Station       EveEntityCategory = "station"
	Invalid       EveEntityCategory = "invalid"
)

func (c EveEntityCategory) Display() string {
	if c == Invalid {
		return "INVALID"
	}
	c2 := strings.ReplaceAll(string(c), "_", " ")
	return cases.Title(language.English).String(c2)
}

type EveEntity struct {
	ID        int32             `json:"id"`
	Category  EveEntityCategory `json:"category"`
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
}

type App struct {
	esiClient *goesi.APIClient
	st        *Storage
}

func NewApp(esiClient *goesi.APIClient, st *Storage) App {
	a := App{
		esiClient: esiClient,
		st:        st,
	}
	return a
}

func (a App) ListCache(ctx context.Context, cmd *cli.Command) error {
	entities, err := a.st.ListEveEntities()
	if err != nil {
		return err
	}
	sortEntities(cmd.String("sort"), entities)
	a.printEveEntitiesWithTimeout(entities)
	return nil
}

func (a App) ClearCache(ctx context.Context, cmd *cli.Command) error {
	n, err := a.st.Clear()
	if err != nil {
		return err
	}
	fmt.Printf("%d objects deleted\n", n)
	return nil
}

func (a App) ResolveIDs(ctx context.Context, cmd *cli.Command) error {
	entities, err := a.resolveIDs(cmd.Int32Args("ID"))
	if err != nil {
		return err
	}
	sortEntities(cmd.String("sort"), entities)
	a.printEveEntities(entities)
	return nil
}

func (a App) resolveIDs(ids []int32) ([]EveEntity, error) {
	entities1, unknownIDs, err := a.resolveIDsFromStorage(ids)
	if err != nil {
		return nil, err
	}
	entities2, err := a.resolveIDsFromAPI(unknownIDs)
	if err != nil {
		return nil, err
	}
	if err := a.st.UpdateOrCreateEveEntities(entities2...); err != nil {
		return nil, err
	}
	m := make(map[int32]EveEntity)
	for _, e := range slices.Concat(entities1, entities2) {
		m[e.ID] = e
	}
	entities := make([]EveEntity, 0)
	for _, id := range ids {
		entities = append(entities, m[id])
	}
	return entities, nil
}

func (a App) resolveIDsFromStorage(ids []int32) ([]EveEntity, []int32, error) {
	entities, err := a.st.ListEveEntitiesByID(ids...)
	if err != nil {
		return nil, nil, err
	}
	found := make(map[int32]bool)
	for _, ee := range entities {
		found[ee.ID] = true
	}
	missing := make([]int32, 0)
	for _, id := range ids {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	return entities, missing, nil
}

func (a App) resolveIDsFromAPI(ids []int32) ([]EveEntity, error) {
	if len(ids) == 0 {
		return []EveEntity{}, nil
	}
	entities, err := a.resolveIDsFromAPI2(ids)
	if errors.Is(err, ErrNotFound) {
		n := len(ids)
		if n == 1 {
			return []EveEntity{{ID: ids[0], Name: "", Category: Invalid}}, nil
		}
		var it1, it2 []EveEntity
		g := new(errgroup.Group)
		g.Go(func() error {
			entities, err := a.resolveIDsFromAPI(ids[:n/2])
			if err != nil {
				return err
			}
			it1 = entities
			return nil
		})
		g.Go(func() error {
			entities, err := a.resolveIDsFromAPI(ids[n/2:])
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
	return entities, nil
}

func (a App) resolveIDsFromAPI2(ids []int32) ([]EveEntity, error) {
	data, res, err := a.esiClient.ESI.UniverseApi.PostUniverseNames(context.Background(), ids, nil)
	if err != nil {
		if res != nil && res.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	eveEntityCategoryFromESICategory := func(c string) EveEntityCategory {
		categoryMap := map[string]EveEntityCategory{
			"alliance":       Alliance,
			"character":      Character,
			"corporation":    Corporation,
			"constellation":  Constellation,
			"faction":        Faction,
			"inventory_type": InventoryType,
			"region":         Region,
			"solar_system":   SolarSystem,
			"station":        Station,
		}
		c2, ok := categoryMap[c]
		if !ok {
			return Undefined
		}
		return c2
	}
	entities := make([]EveEntity, 0)
	for _, o := range data {
		entities = append(entities, EveEntity{
			ID:       o.Id,
			Name:     o.Name,
			Category: eveEntityCategoryFromESICategory(o.Category),
		})
	}
	return entities, nil
}

func (a App) ResolveNames(ctx context.Context, cmd *cli.Command) error {
	entities1, missing, err := a.resolveNamesFromStorage(cmd.StringArgs("Name"))
	if err != nil {
		return err
	}
	entities2, err := a.resolveNamesFromAPI(missing)
	if err != nil {
		return err
	}
	entities := slices.Concat(entities1, entities2)
	sortEntities(cmd.String("sort"), entities)
	a.printEveEntities(entities)
	return nil
}

func (a App) resolveNamesFromStorage(names []string) ([]EveEntity, []string, error) {
	entities, err := a.st.ListEveEntitiesByName(names...)
	if err != nil {
		return nil, nil, err
	}
	found := make(map[string]bool)
	for _, ee := range entities {
		found[ee.Name] = true
	}
	missing := make([]string, 0)
	for _, n := range names {
		if !found[n] {
			missing = append(missing, n)
		}
	}
	return entities, missing, nil
}

func (a App) resolveNamesFromAPI(names []string) ([]EveEntity, error) {
	if len(names) == 0 {
		return []EveEntity{}, nil
	}
	data, res, err := a.esiClient.ESI.UniverseApi.PostUniverseIds(context.Background(), names, nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error: %s", res.Status)
	}
	matches := make(map[string]bool)
	for _, n := range names {
		matches[n] = true
	}
	found := make(map[string]bool)
	entities := make([]EveEntity, 0)
	addEntity := func(id int32, name string, category EveEntityCategory) {
		if !matches[name] {
			return
		}
		entities = append(entities, EveEntity{
			ID:       id,
			Name:     name,
			Category: category,
		})
		found[name] = true
	}
	for _, o := range data.Agents {
		addEntity(o.Id, o.Name, Agent)
	}
	for _, o := range data.Alliances {
		addEntity(o.Id, o.Name, Alliance)
	}
	for _, o := range data.Characters {
		addEntity(o.Id, o.Name, Character)
	}
	for _, o := range data.Constellations {
		addEntity(o.Id, o.Name, Constellation)
	}
	for _, o := range data.Corporations {
		addEntity(o.Id, o.Name, Corporation)
	}
	for _, o := range data.Factions {
		addEntity(o.Id, o.Name, Faction)
	}
	for _, o := range data.InventoryTypes {
		addEntity(o.Id, o.Name, InventoryType)
	}
	for _, o := range data.Regions {
		addEntity(o.Id, o.Name, Region)
	}
	for _, o := range data.Stations {
		addEntity(o.Id, o.Name, Station)
	}
	for _, o := range data.Systems {
		addEntity(o.Id, o.Name, SolarSystem)
	}
	for _, n := range names {
		if found[n] {
			continue
		}
		entities = append(entities, EveEntity{Name: n, Category: Invalid})
	}
	if err := a.st.UpdateOrCreateEveEntities(entities...); err != nil {
		return nil, err
	}
	return entities, nil
}

func sortEntities(column string, entities []EveEntity) {
	slices.SortFunc(entities, func(a, b EveEntity) int {
		switch column {
		case "category":
			if a.Category != b.Category {
				return strings.Compare(strings.ToLower(string(a.Category)), strings.ToLower(string(b.Category)))
			}
			return cmp.Compare(a.ID, b.ID)
		case "name":
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case "timestamp":
			return a.Timestamp.Compare(b.Timestamp)
		default:
			return cmp.Compare(a.ID, b.ID)
		}
	})
}

func (a App) printEveEntities(entities []EveEntity) {
	a.printEveEntitiesX(entities, false)
}

func (a App) printEveEntitiesWithTimeout(entities []EveEntity) {
	a.printEveEntitiesX(entities, true)
}

func (App) printEveEntitiesX(entities []EveEntity, showTimestamp bool) {
	data := make([][]any, 0)
	for _, ee := range entities {
		if showTimestamp {
			data = append(data, []any{ee.ID, ee.Name, ee.Category.Display(), ee.Timestamp.Format(time.RFC3339)})
		} else {
			data = append(data, []any{ee.ID, ee.Name, ee.Category.Display()})
		}
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
	if showTimestamp {
		table.Header("ID", "Name", "Category", "Timestamp")
	} else {
		table.Header("ID", "Name", "Category")
	}
	table.Bulk(data)
	table.Render()
}
