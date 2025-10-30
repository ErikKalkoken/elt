package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/antihax/goesi"
	"github.com/antihax/goesi/esi"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
)

const (
	nameInvalid = "INVALID"
)

type result struct {
	category EveEntityCategory
	table    *tablewriter.Table
}

type App struct {
	// Whether to show the spinner
	SpinnerDisabled bool

	// Max width of the terminal in characters.
	MaxWidth int

	esiClient *goesi.APIClient
	out       io.Writer
	st        *Storage
}

func NewApp(esiClient *goesi.APIClient, st *Storage, out io.Writer) App {
	a := App{
		esiClient: esiClient,
		out:       out,
		st:        st,
	}
	return a
}

// Run is the main entry point.
func (a App) Run(args []string) error {
	// Parse args
	var (
		ids     []int32
		invalid []int
		names   []string
	)
	for _, arg := range args {
		id, err := strconv.Atoi(arg)
		if err != nil {
			names = append(names, arg)
		} else {
			id32 := int32(id)
			if int(id32) != id || id == 0 {
				invalid = append(invalid, id)
				continue
			}
			ids = append(ids, id32)
		}
	}
	if len(invalid) > 0 {
		fmt.Fprintf(a.out, "Ignoring invalid IDs: %v\n", invalid)
	}

	if len(ids) == 0 && len(names) == 0 {
		return fmt.Errorf("no suitable input to process")
	}

	// Resolve ids and names
	var bar *progressbar.ProgressBar
	if !a.SpinnerDisabled {
		bar = progressbar.NewOptions(-1,
			progressbar.OptionSpinnerType(14), // choose spinner style (0–39)
			progressbar.OptionSetDescription("Processing..."),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionSetWriter(a.out),
		)
	}
	g := new(errgroup.Group)
	var oo1, oo2 []EveEntity
	if len(ids) > 0 {
		g.Go(func() error {
			oo, err := a.resolveIDs(ids)
			if err != nil {
				return err
			}
			oo1 = oo
			return nil
		})
	}
	if len(names) > 0 {
		g.Go(func() error {
			oo, err := a.resolveNames(names)
			if err != nil {
				return err
			}
			oo2 = oo
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	oo := slices.Concat(oo1, oo2)

	// build results
	results, err := a.buildResults(oo)
	if err != nil {
		return err
	}

	if bar != nil {
		bar.Clear()
	}

	// Print results
	for _, r := range results {
		fmt.Fprintln(a.out, r.category.Display()+":")
		r.table.Render()
	}
	return nil
}

func (a App) resolveIDs(ids []int32) ([]EveEntity, error) {
	entities1, unknownIDs, err := a.st.ListFreshEveEntityByID(ids)
	if err != nil {
		return nil, err
	}
	entities2, err := a.resolveIDsFromAPI(unknownIDs)
	if err != nil {
		return nil, err
	}
	entities3 := slices.DeleteFunc(slices.Clone(entities2), func(o EveEntity) bool {
		return o.ID() == 0
	})
	if err := a.st.UpdateOrCreateEveEntity(entities3); err != nil {
		return nil, err
	}
	m := make(map[int32]EveEntity)
	for _, e := range slices.Concat(entities1, entities2) {
		m[e.EntityID] = e
	}
	entities := make([]EveEntity, 0)
	for _, id := range ids {
		entities = append(entities, m[id])
	}
	return entities, nil
}

func (a App) resolveIDsFromAPI(ids []int32) ([]EveEntity, error) {
	if len(ids) == 0 {
		return []EveEntity{}, nil
	}
	entities, err := a.resolveIDsFromAPI2(ids)
	if errors.Is(err, ErrNotFound) {
		n := len(ids)
		if n == 1 {
			return []EveEntity{{
				EntityID:  ids[0],
				Name:      "",
				Category:  CategoryInvalid,
				Timestamp: now(),
			}}, nil
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
	data, r, err := a.esiClient.ESI.UniverseApi.PostUniverseNames(context.Background(), ids, nil)
	if err != nil {
		if r != nil && r.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	eveEntityCategoryFromESICategory := func(c string) EveEntityCategory {
		categoryMap := map[string]EveEntityCategory{
			"alliance":       CategoryAlliance,
			"character":      CategoryCharacter,
			"corporation":    CategoryCorporation,
			"constellation":  CategoryConstellation,
			"faction":        CategoryFaction,
			"inventory_type": CategoryInventoryType,
			"region":         CategoryRegion,
			"solar_system":   CategorySolarSystem,
			"station":        CategoryStation,
		}
		c2, ok := categoryMap[c]
		if !ok {
			return CategoryUnknown
		}
		return c2
	}
	entities := make([]EveEntity, 0)
	for _, o := range data {
		entities = append(entities, EveEntity{
			EntityID:  o.Id,
			Name:      o.Name,
			Category:  eveEntityCategoryFromESICategory(o.Category),
			Timestamp: now(),
		})
	}
	return entities, nil
}

func (a App) resolveNames(names []string) ([]EveEntity, error) {
	if len(names) == 0 {
		return []EveEntity{}, nil
	}
	data, r, err := a.esiClient.ESI.UniverseApi.PostUniverseIds(context.Background(), names, nil)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error: %s", r.Status)
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
			EntityID:  id,
			Name:      name,
			Category:  category,
			Timestamp: now(),
		})
		found[name] = true
	}
	for _, o := range data.Agents {
		addEntity(o.Id, o.Name, CategoryAgent)
	}
	for _, o := range data.Alliances {
		addEntity(o.Id, o.Name, CategoryAlliance)
	}
	for _, o := range data.Characters {
		addEntity(o.Id, o.Name, CategoryCharacter)
	}
	for _, o := range data.Constellations {
		addEntity(o.Id, o.Name, CategoryConstellation)
	}
	for _, o := range data.Corporations {
		addEntity(o.Id, o.Name, CategoryCorporation)
	}
	for _, o := range data.Factions {
		addEntity(o.Id, o.Name, CategoryFaction)
	}
	for _, o := range data.InventoryTypes {
		addEntity(o.Id, o.Name, CategoryInventoryType)
	}
	for _, o := range data.Regions {
		addEntity(o.Id, o.Name, CategoryRegion)
	}
	for _, o := range data.Stations {
		addEntity(o.Id, o.Name, CategoryStation)
	}
	for _, o := range data.Systems {
		addEntity(o.Id, o.Name, CategorySolarSystem)
	}
	for _, n := range names {
		if found[n] {
			continue
		}
		entities = append(entities, EveEntity{
			Name:      n,
			Category:  CategoryInvalid,
			Timestamp: now(),
		})
	}
	entities2 := slices.DeleteFunc(slices.Clone(entities), func(o EveEntity) bool {
		return o.ID() == 0
	})
	if err := a.st.UpdateOrCreateEveEntity(entities2); err != nil {
		return nil, err
	}
	return entities, nil
}

func (a App) buildResults(entities []EveEntity) ([]result, error) {
	category2IDs := make(map[EveEntityCategory][]int32)
	for _, e := range entities {
		category2IDs[e.Category] = append(category2IDs[e.Category], e.ID())
	}
	results := make([]result, len(category2IDs))
	g := new(errgroup.Group)
	for i, c := range slices.Sorted(maps.Keys(category2IDs)) {
		g.Go(func() error {
			ids := category2IDs[c]
			switch c {
			case CategoryAgent:
				t, err := a.buildCharacterTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategoryAlliance:
				t, err := a.buildAllianceTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategoryCharacter:
				t, err := a.buildCharacterTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategoryConstellation:
				t, err := a.buildConstellationTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategoryCorporation:
				t, err := a.buildCorporationTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategoryFaction:
				t, err := a.buildFactionTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategoryInventoryType:
				t, err := a.buildTypeTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategoryRegion:
				t, err := a.buildRegionTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategorySolarSystem:
				t, err := a.buildSolarSystemTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategoryStation:
				t, err := a.buildStationTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{c, t}
			case CategoryInvalid:
				entities2 := slices.DeleteFunc(entities, func(o EveEntity) bool {
					return o.Category != CategoryInvalid
				})
				t := makeSortedTable(
					a,
					[]string{"ID", "Name", "Category"},
					entities2, func(o EveEntity) []any {
						return []any{o.EntityID, o.Name, o.Category.Display()}
					},
				)
				results[i] = result{c, t}
			default:
				entities, _, err := a.st.ListFreshEveEntityByID(ids)
				if err != nil {
					return err
				}
				t := makeSortedTable(
					a,
					[]string{"ID", "Name", "Category"},
					entities,
					func(o EveEntity) []any {
						return []any{o.EntityID, o.Name, o.Category.Display()}
					},
				)
				results[i] = result{c, t}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	var results2 []result
	for _, r := range results {
		if r.table == nil {
			continue
		}
		results2 = append(results2, r)
	}
	return results2, nil
}

func (a App) buildCharacterTable(ids []int32) (*tablewriter.Table, error) {
	characters, err := a.fetchCharacters(ids)
	if err != nil {
		return nil, err
	}
	var entityIDs []int32
	for _, o := range characters {
		entityIDs = append(entityIDs, o.CorporationID)
		if o.AllianceID != 0 {
			entityIDs = append(entityIDs, o.AllianceID)
		}
	}
	ee, err := a.resolveIDs(entityIDs)
	if err != nil {
		return nil, err
	}
	entityLookup := makeLookupMap(ee)
	t := makeSortedTable(
		a,
		[]string{"ID", "Name", "CorporationID", "CorporationName", "AllianceID", "AllianceName", "NPC"},
		characters,
		func(o EveCharacter) []any {
			corporationName := entityLookup[o.CorporationID].Name
			return []any{o.ID(), o.Name, o.CorporationID, corporationName, idOrEmpty(o.AllianceID), entityLookup[o.AllianceID].Name, o.IsNPC()}
		})
	return t, nil
}

func (a App) fetchCharacters(ids []int32) ([]EveCharacter, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveCharacterByID,
		func(id int32) (esi.GetCharactersCharacterIdOk, *http.Response, error) {
			return a.esiClient.ESI.CharacterApi.GetCharactersCharacterId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetCharactersCharacterIdOk) EveCharacter {
			return EveCharacter{
				AllianceID:    x.AllianceId,
				CharacterID:   id,
				CorporationID: x.CorporationId,
				Name:          x.Name,
				Timestamp:     now(),
			}
		},
		a.st.UpdateOrCreateEveCharacter,
	)
	return oo, err
}

func (a App) buildCorporationTable(ids []int32) (*tablewriter.Table, error) {
	corporations, err := a.fetchCorporations(ids)
	if err != nil {
		return nil, err
	}
	var entityIDs []int32
	for _, o := range corporations {
		if o.AllianceID != 0 {
			entityIDs = append(entityIDs, o.AllianceID)
		}
	}
	entities, err := a.resolveIDs(entityIDs)
	if err != nil {
		return nil, err
	}
	entityLookup := makeLookupMap(entities)
	t := makeSortedTable(
		a,
		[]string{"ID", "Name", "Ticker", "Members", "AllianceID", "AllianceName", "NPC"},
		corporations,
		func(o EveCorporation) []any {
			return []any{o.ID(), o.Name, o.Ticker, o.MemberCount, idOrEmpty(o.AllianceID), entityLookup[o.AllianceID].Name, o.IsNPC()}
		})
	return t, err
}

func (a App) fetchCorporations(ids []int32) ([]EveCorporation, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveCorporationByID,
		func(id int32) (esi.GetCorporationsCorporationIdOk, *http.Response, error) {
			return a.esiClient.ESI.CorporationApi.GetCorporationsCorporationId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetCorporationsCorporationIdOk) EveCorporation {
			return EveCorporation{
				AllianceID:    x.AllianceId,
				CeoID:         x.CeoId,
				CorporationID: id,
				MemberCount:   x.MemberCount,
				Name:          x.Name,
				Ticker:        x.Ticker,
				Timestamp:     now(),
			}
		},
		a.st.UpdateOrCreateEveCorporation,
	)
	return oo, err
}

func (a App) buildAllianceTable(ids []int32) (*tablewriter.Table, error) {
	alliances, err := a.fetchAlliance(ids)
	if err != nil {
		return nil, err
	}
	t := makeSortedTable(
		a,
		[]string{"ID", "Name", "Ticker"},
		alliances,
		func(o EveAlliance) []any {
			return []any{o.ID(), o.Name, o.Ticker}
		})
	return t, nil
}

func (a App) fetchAlliance(ids []int32) ([]EveAlliance, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveAllianceByID,
		func(id int32) (esi.GetAlliancesAllianceIdOk, *http.Response, error) {
			return a.esiClient.ESI.AllianceApi.GetAlliancesAllianceId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetAlliancesAllianceIdOk) EveAlliance {
			return EveAlliance{
				AllianceID: id,
				Name:       x.Name,
				Ticker:     x.Ticker,
				Timestamp:  now(),
			}
		},
		a.st.UpdateOrCreateEveAlliance,
	)
	return oo, err
}

func (a App) buildFactionTable(ids []int32) (*tablewriter.Table, error) {
	factions, err := a.fetchFactions(ids)
	if err != nil {
		return nil, err
	}
	var entityIDs []int32
	for _, o := range factions {
		if o.CorporationID != 0 {
			entityIDs = append(entityIDs, o.CorporationID)
		}
		if o.MilitiaCorporationID != 0 {
			entityIDs = append(entityIDs, o.MilitiaCorporationID)
		}
	}
	entities, err := a.resolveIDs(entityIDs)
	if err != nil {
		return nil, err
	}
	entityLookup := makeLookupMap(entities)
	t := makeSortedTable(
		a,
		[]string{"ID", "Name", "CorporationID", "CorporationName", "MilitiaCorporationID", "MilitiaCorporationName"},
		factions,
		func(o EveFaction) []any {
			return []any{o.ID(), o.Name, idOrEmpty(o.CorporationID), entityLookup[o.CorporationID].Name, idOrEmpty(o.MilitiaCorporationID), entityLookup[o.MilitiaCorporationID].Name}
		})
	return t, nil
}

func (a App) fetchFactions(ids []int32) ([]EveFaction, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveFactionByID,
		func(id int32) ([]esi.GetUniverseFactions200Ok, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseFactions(context.Background(), nil)
		},
		func(id int32, xx []esi.GetUniverseFactions200Ok) EveFaction {
			for _, x := range xx {
				if x.FactionId != id {
					continue
				}
				return EveFaction{
					FactionID:            id,
					CorporationID:        x.CorporationId,
					MilitiaCorporationID: x.MilitiaCorporationId,
					Name:                 x.Name,
					Timestamp:            now(),
				}
			}
			return EveFaction{
				FactionID: id,
				Name:      nameInvalid,
				Timestamp: now(),
			}
		},
		a.st.UpdateOrCreateEveFaction,
	)
	return oo, err
}

func (a App) buildStationTable(ids []int32) (*tablewriter.Table, error) {
	stations, err := a.fetchStations(ids)
	if err != nil {
		return nil, err
	}
	var entityIDs []int32
	for _, et := range stations {
		entityIDs = append(entityIDs, et.OwnerID)
		entityIDs = append(entityIDs, et.SolarSystemID)
		entityIDs = append(entityIDs, et.TypeID)
	}
	entities, err := a.resolveIDs(entityIDs)
	if err != nil {
		return nil, err
	}
	entityLookup := makeLookupMap(entities)
	t := makeSortedTable(
		a,
		[]string{"ID", "Name", "SolarSystemID", "SolarSystemName", "TypeID", "TypeName", "OwnerID", "OwnerName"},
		stations,
		func(o EveStation) []any {
			typeName := entityLookup[o.TypeID].Name
			ownerName := entityLookup[o.OwnerID].Name
			solarSystemName := entityLookup[o.SolarSystemID].Name
			return []any{o.StationID, o.Name, o.SolarSystemID, solarSystemName, o.TypeID, typeName, o.OwnerID, ownerName}
		})
	return t, nil
}

func (a App) fetchStations(ids []int32) ([]EveStation, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveStationByID,
		func(id int32) (esi.GetUniverseStationsStationIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseStationsStationId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetUniverseStationsStationIdOk) EveStation {
			return EveStation{
				Name:          x.Name,
				OwnerID:       x.Owner,
				SolarSystemID: x.SystemId,
				StationID:     id,
				Timestamp:     now(),
				TypeID:        x.TypeId,
			}
		},
		a.st.UpdateOrCreateEveStation,
	)
	return oo, err
}

func (a App) buildTypeTable(ids []int32) (*tablewriter.Table, error) {
	types, err := a.fetchTypes(ids)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]int32, 0)
	for _, et := range types {
		groupIDs = append(groupIDs, et.GroupID)
	}
	groups, err := a.fetchGroups(groupIDs)
	if err != nil {
		return nil, err
	}
	groupLookup := makeLookupMap(groups)
	categoryIDs := make([]int32, 0)
	for _, eg := range groups {
		categoryIDs = append(categoryIDs, eg.CategoryID)
	}
	categories, err := a.fetchCategories(categoryIDs)
	if err != nil {
		return nil, err
	}
	categoryLookup := makeLookupMap(categories)
	t := makeSortedTable(
		a,
		[]string{"ID", "Name", "GroupID", "GroupName", "CategoryID", "CategoryName", "Published"},
		types,
		func(o EveType) []any {
			group := groupLookup[o.GroupID]
			category := categoryLookup[group.CategoryID]
			return []any{o.TypeID, o.Name, group.GroupID, group.Name, category.CategoryID, category.Name, o.Published}
		})
	return t, nil
}

func (a App) fetchTypes(ids []int32) ([]EveType, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveTypeByID,
		func(id int32) (esi.GetUniverseTypesTypeIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseTypesTypeId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetUniverseTypesTypeIdOk) EveType {
			return EveType{
				GroupID:   x.GroupId,
				TypeID:    id,
				Name:      x.Name,
				Published: x.Published,
				Timestamp: now(),
			}
		},
		a.st.UpdateOrCreateEveType,
	)
	return oo, err
}

func (a App) fetchCategories(ids []int32) ([]EveCategory, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveCategoryByID,
		func(id int32) (esi.GetUniverseCategoriesCategoryIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseCategoriesCategoryId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetUniverseCategoriesCategoryIdOk) EveCategory {
			return EveCategory{
				CategoryID: id,
				Name:       x.Name,
				Published:  x.Published,
				Timestamp:  now(),
			}
		},
		a.st.UpdateOrCreateEveCategory,
	)
	return oo, err
}

func (a App) fetchGroups(ids []int32) ([]EveGroup, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveGroupByID,
		func(id int32) (esi.GetUniverseGroupsGroupIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseGroupsGroupId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetUniverseGroupsGroupIdOk) EveGroup {
			return EveGroup{
				CategoryID: x.CategoryId,
				GroupID:    id,
				Name:       x.Name,
				Published:  x.Published,
				Timestamp:  now(),
			}
		},
		a.st.UpdateOrCreateEveGroup,
	)
	return oo, err
}

func (a App) buildSolarSystemTable(ids []int32) (*tablewriter.Table, error) {
	types, err := a.fetchSolarSystems(ids)
	if err != nil {
		return nil, err
	}
	constellationIDs := make([]int32, 0)
	for _, o := range types {
		constellationIDs = append(constellationIDs, o.ConstellationID)
	}
	constellations, err := a.fetchConstellations(constellationIDs)
	if err != nil {
		return nil, err
	}
	constellationLookup := makeLookupMap(constellations)
	regionIDs := make([]int32, 0)
	for _, o := range constellations {
		regionIDs = append(regionIDs, o.RegionID)
	}
	regions, err := a.fetchRegions(regionIDs)
	if err != nil {
		return nil, err
	}
	regionLookup := makeLookupMap(regions)
	t := makeSortedTable(
		a,
		[]string{"ID", "Name", "ConstellationID", "ConstellationName", "RegionID", "RegionName", "Security"},
		types,
		func(o EveSolarSystem) []any {
			constellation := constellationLookup[o.ConstellationID]
			region := regionLookup[constellation.RegionID]
			return []any{o.ID(), o.Name, constellation.ConstellationID, constellation.Name, region.RegionID, region.Name, o.Security}
		})
	return t, nil
}

func (a App) fetchSolarSystems(ids []int32) ([]EveSolarSystem, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveSolarSystemByID,
		func(id int32) (esi.GetUniverseSystemsSystemIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseSystemsSystemId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetUniverseSystemsSystemIdOk) EveSolarSystem {
			return EveSolarSystem{
				ConstellationID: x.ConstellationId,
				Name:            x.Name,
				Security:        x.SecurityStatus,
				SolarSystemID:   id,
				Timestamp:       now(),
			}
		},
		a.st.UpdateOrCreateEveSolarSystem,
	)
	return oo, err
}

func (a App) buildConstellationTable(ids []int32) (*tablewriter.Table, error) {
	constellations, err := a.fetchConstellations(ids)
	if err != nil {
		return nil, err
	}
	regionIDs := make([]int32, 0)
	for _, o := range constellations {
		regionIDs = append(regionIDs, o.RegionID)
	}
	regions, err := a.fetchRegions(regionIDs)
	if err != nil {
		return nil, err
	}
	regionLookup := makeLookupMap(regions)
	t := makeSortedTable(
		a,
		[]string{"ID", "Name", "RegionID", "RegionName"},
		constellations,
		func(o EveConstellation) []any {
			return []any{o.ID(), o.Name, o.RegionID, regionLookup[o.RegionID].Name}
		})
	return t, nil
}

func (a App) fetchConstellations(ids []int32) ([]EveConstellation, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveConstellationByID,
		func(id int32) (esi.GetUniverseConstellationsConstellationIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseConstellationsConstellationId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetUniverseConstellationsConstellationIdOk) EveConstellation {
			return EveConstellation{
				ConstellationID: id,
				RegionID:        x.RegionId,
				Name:            x.Name,
				Timestamp:       now(),
			}
		},
		a.st.UpdateOrCreateEveConstellation,
	)
	return oo, err
}

func (a App) buildRegionTable(ids []int32) (*tablewriter.Table, error) {
	regions, err := a.fetchRegions(ids)
	if err != nil {
		return nil, err
	}
	t := makeSortedTable(
		a,
		[]string{"ID", "Name"},
		regions,
		func(o EveRegion) []any {
			return []any{o.ID(), o.Name}
		},
	)
	return t, nil
}

func (a App) fetchRegions(ids []int32) ([]EveRegion, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveRegionByID,
		func(id int32) (esi.GetUniverseRegionsRegionIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseRegionsRegionId(context.Background(), id, nil)
		},
		func(id int32, x esi.GetUniverseRegionsRegionIdOk) EveRegion {
			return EveRegion{
				RegionID:  id,
				Name:      x.Name,
				Timestamp: now(),
			}
		},
		a.st.UpdateOrCreateEveRegion,
	)
	return oo, err
}

func idOrEmpty(id int32) string {
	if id == 0 {
		return ""
	}
	return strconv.Itoa(int(id))
}

func sliceUnique[T comparable](s []T) []T {
	m := make(map[T]bool)
	for _, v := range s {
		m[v] = true
	}
	return slices.Collect(maps.Keys(m))
}

func makeLookupMap[T EveObject](objs []T) map[int32]T {
	m := make(map[int32]T)
	for _, o := range objs {
		m[o.ID()] = o
	}
	return m
}

// fetchObjects fetches and returns eve objects for the given ids.
// It returns objects from storage when found or otherwise fetches them from the API.
// It also returns a slice of invalid IDs for objects which could not be found.
func fetchObjects[X any, Y EveObject](ids []int32, fetcherStorage func([]int32) ([]Y, []int32, error), fetcherAPI func(id int32) (X, *http.Response, error), mapper func(id int32, x X) Y, storer func([]Y) error) ([]Y, []int32, error) {
	wrapErr := func(err error) error {
		var z Y
		return fmt.Errorf("fetch objects %T: %v: %w", z, ids, err)
	}
	objsLocal, missing, err := fetcherStorage(sliceUnique(ids))
	if err != nil {
		return nil, nil, wrapErr(err)
	}
	objsRemote := make([]Y, len(missing))
	invalidIDs := make([]int32, len(missing))
	g := new(errgroup.Group)
	for i, id := range missing {
		g.Go(func() error {
			x, r, err := fetcherAPI(id)
			if err != nil {
				if r != nil && r.StatusCode == http.StatusNotFound {
					invalidIDs[i] = id
					return nil
				}
				return err
			}
			objsRemote[i] = mapper(id, x)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, nil, wrapErr(err)
	}
	if len(objsRemote) > 0 {
		err := storer(objsRemote)
		if err != nil {
			return nil, nil, wrapErr(err)
		}
	}
	invalid2 := slices.DeleteFunc(invalidIDs, func(x int32) bool {
		return x == 0
	})
	objs := slices.Concat(objsLocal, objsRemote)
	return objs, invalid2, nil
}

func makeSortedTable[T EveObject](a App, headers []string, objs []T, makeRow func(T) []any) *tablewriter.Table {
	slices.SortFunc(objs, func(a, b T) int {
		return cmp.Compare(a.ID(), b.ID())
	})
	rows := make([][]any, 0)
	for _, o := range objs {
		rows = append(rows, makeRow(o))
	}
	t := tablewriter.NewTable(a.out,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Settings: tw.Settings{Separators: tw.Separators{BetweenRows: tw.On}},
		})),
		tablewriter.WithConfig(tablewriter.Config{
			MaxWidth: a.MaxWidth,
			Row: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoWrap: tw.WrapNormal},
				Alignment:  tw.CellAlignment{Global: tw.AlignLeft}, // Left-align rows
			},
		}),
	)
	t.Header(headers)
	t.Bulk(rows)
	return t
}

func now() time.Time {
	return time.Now().UTC()
}
