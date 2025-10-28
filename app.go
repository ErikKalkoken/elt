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
	"time"

	"github.com/antihax/goesi"
	"github.com/antihax/goesi/esi"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
)

const (
	nameInvalid = "INVALID"
)

type App struct {
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

func (a App) DumpCache(ctx context.Context, cmd *cli.Command) error {
	entities, err := a.st.ListEveEntities()
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, "Entities")
	printTableWithSort(a.out, []string{"ID", "Name", "Category", "Timeout"}, entities, func(ee EveEntity) []any {
		return []any{ee.EntityID, ee.Name, ee.Category.Display(), ee.Timestamp.Format(time.RFC3339)}
	})

	categories, err := a.st.ListEveCategories()
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, "Categories")
	printTableWithSort(a.out, []string{"ID", "Name", "Timestamp"}, categories, func(o EveCategory) []any {
		return []any{o.CategoryID, o.Name, o.Timestamp.Format(time.RFC3339)}
	})

	groups, err := a.st.ListEveGroups()
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, "Groups")
	printTableWithSort(a.out, []string{"ID", "Name", "CategoryID", "Timestamp"}, groups, func(o EveGroup) []any {
		return []any{o.GroupID, o.Name, o.CategoryID, o.Timestamp.Format(time.RFC3339)}
	})

	types, err := a.st.ListEveTypes()
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, "Types")
	printTableWithSort(a.out, []string{"ID", "Name", "GroupID", "Timestamp"}, types, func(o EveType) []any {
		return []any{o.TypeID, o.Name, o.GroupID, o.Timestamp.Format(time.RFC3339)}
	})
	return nil
}

func (a App) ClearCache(ctx context.Context, cmd *cli.Command) error {
	n, err := a.st.Clear()
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%d objects deleted\n", n)
	return nil
}

func (a App) ResolveIDs(ctx context.Context, cmd *cli.Command) error {
	entities, err := a.resolveIDs(cmd.Int32Args("ID"))
	if err != nil {
		return err
	}
	if err := a.fetchAndPrintResults(entities); err != nil {
		return err
	}
	return nil
}

func (a App) fetchAndPrintResults(entities []EveEntity) error {
	category2IDs := make(map[EveEntityCategory][]int32)
	for _, e := range entities {
		category2IDs[e.Category] = append(category2IDs[e.Category], e.ID())
	}
	for _, c := range slices.Sorted(maps.Keys(category2IDs)) {
		fmt.Fprintln(a.out, c.Display()+":")
		ids := category2IDs[c]
		switch c {
		case Character:
			err := a.fetchAndPrintCharacters(ids)
			if err != nil {
				return err
			}
		case InventoryType:
			err := a.fetchAndPrintTypes(ids)
			if err != nil {
				return err
			}
		case Invalid:
			entities2 := slices.DeleteFunc(entities, func(o EveEntity) bool {
				return o.Category != Invalid
			})
			printTableWithSort(a.out, []string{"ID", "Name", "Category"}, entities2, func(o EveEntity) []any {
				return []any{o.EntityID, o.Name, o.Category.Display()}
			})

		default:
			entities, _, err := a.st.ListFreshEveEntitiesByID(ids)
			if err != nil {
				return err
			}
			printTableWithSort(a.out, []string{"ID", "Name", "Category"}, entities, func(o EveEntity) []any {
				return []any{o.EntityID, o.Name, o.Category.Display()}
			})
		}
	}
	return nil
}

func (a App) resolveIDs(ids []int32) ([]EveEntity, error) {
	entities1, unknownIDs, err := a.st.ListFreshEveEntitiesByID(ids)
	if err != nil {
		return nil, err
	}
	entities2, err := a.resolveIDsFromAPI(unknownIDs)
	if err != nil {
		return nil, err
	}
	if err := a.st.UpdateOrCreateEveEntities(entities2); err != nil {
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
				Category:  Invalid,
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
			EntityID:  o.Id,
			Name:      o.Name,
			Category:  eveEntityCategoryFromESICategory(o.Category),
			Timestamp: now(),
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
	if err := a.fetchAndPrintResults(entities); err != nil {
		return err
	}
	return nil
}

func (a App) resolveNamesFromStorage(names []string) ([]EveEntity, []string, error) {
	entities, err := a.st.ListFreshEveEntitiesByName(sliceUnique(names))
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
		entities = append(entities, EveEntity{
			Name:      n,
			Category:  Invalid,
			Timestamp: now(),
		})
	}
	entities2 := slices.DeleteFunc(slices.Clone(entities), func(o EveEntity) bool {
		return o.ID() == 0
	})
	if err := a.st.UpdateOrCreateEveEntities(entities2); err != nil {
		return nil, err
	}
	return entities, nil
}

func (a App) fetchAndPrintCharacters(ids []int32) error {
	characters, err := a.fetchCharacters(ids)
	if err != nil {
		return err
	}
	groupIDs := make([]int32, 0)
	for _, et := range characters {
		groupIDs = append(groupIDs, et.GroupID)
	}
	groups, err := a.fetchGroups(groupIDs)
	if err != nil {
		return err
	}
	categoryIDs := make([]int32, 0)
	for _, eg := range groups {
		categoryIDs = append(categoryIDs, eg.CategoryID)
	}
	categories, err := a.fetchCategories(categoryIDs)
	if err != nil {
		return err
	}
	categoryLookup := makeLookupMap(categories)
	groupLookup := makeLookupMap(groups)
	printTableWithSort(a.out, []string{"ID", "Name", "GroupID", "GroupName", "CategoryID", "CategoryName", "Published"}, characters, func(o EveType) []any {
		group := groupLookup[o.GroupID]
		category := categoryLookup[group.CategoryID]
		return []any{o.TypeID, o.Name, group.GroupID, group.Name, category.CategoryID, category.Name, o.Published}
	})
	return nil
}

func (a App) fetchCharacters(ids []int32) ([]EveType, error) {
	return nil, nil
}

func (a App) fetchAndPrintTypes(ids []int32) error {
	types, err := a.fetchTypes(ids)
	if err != nil {
		return err
	}
	groupIDs := make([]int32, 0)
	for _, et := range types {
		groupIDs = append(groupIDs, et.GroupID)
	}
	groups, err := a.fetchGroups(groupIDs)
	if err != nil {
		return err
	}
	categoryIDs := make([]int32, 0)
	for _, eg := range groups {
		categoryIDs = append(categoryIDs, eg.CategoryID)
	}
	categories, err := a.fetchCategories(categoryIDs)
	if err != nil {
		return err
	}
	categoryLookup := makeLookupMap(categories)
	groupLookup := makeLookupMap(groups)
	printTableWithSort(a.out, []string{"ID", "Name", "GroupID", "GroupName", "CategoryID", "CategoryName", "Published"}, types, func(o EveType) []any {
		group := groupLookup[o.GroupID]
		category := categoryLookup[group.CategoryID]
		return []any{o.TypeID, o.Name, group.GroupID, group.Name, category.CategoryID, category.Name, o.Published}
	})
	return nil
}

func (a App) fetchTypes(ids []int32) ([]EveType, error) {
	typesLocal, missing, err := a.st.ListFreshEveTypesByID(sliceUnique(ids))
	if err != nil {
		return nil, err
	}
	typesRemote, err := fetchObjectsFromAPI(
		missing,
		func(id int32) (esi.GetUniverseTypesTypeIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseTypesTypeId(context.Background(), id, nil)
		},
		func(x esi.GetUniverseTypesTypeIdOk) EveType {
			return EveType{
				GroupID:   x.GroupId,
				TypeID:    x.TypeId,
				Name:      x.Name,
				Published: x.Published,
				Timestamp: now(),
			}
		},
		func(id int32) EveType {
			return EveType{
				TypeID:    id,
				Name:      nameInvalid,
				Timestamp: now(),
			}
		},
	)
	if err != nil {
		return nil, err
	}
	if len(typesRemote) > 0 {
		err := a.st.UpdateOrCreateEveTypes(typesRemote)
		if err != nil {
			return nil, err
		}
	}
	types := slices.Concat(typesLocal, typesRemote)
	return types, nil
}

func (a App) fetchCategories(ids []int32) ([]EveCategory, error) {
	groupsLocal, missing, err := a.st.ListFreshEveCategoriesByID(sliceUnique(ids)...)
	if err != nil {
		return nil, err
	}
	groupsRemote, err := fetchObjectsFromAPI(
		missing,
		func(id int32) (esi.GetUniverseCategoriesCategoryIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseCategoriesCategoryId(context.Background(), id, nil)
		},
		func(x esi.GetUniverseCategoriesCategoryIdOk) EveCategory {
			return EveCategory{
				CategoryID: x.CategoryId,
				Name:       x.Name,
				Published:  x.Published,
				Timestamp:  now(),
			}
		},
		func(id int32) EveCategory {
			return EveCategory{
				CategoryID: id,
				Name:       nameInvalid,
				Timestamp:  now(),
			}
		},
	)
	if len(groupsRemote) > 0 {
		err := a.st.UpdateOrCreateEveCategories(groupsRemote)
		if err != nil {
			return nil, err
		}
	}
	groups := slices.Concat(groupsLocal, groupsRemote)
	return groups, err
}

func (a App) fetchGroups(ids []int32) ([]EveGroup, error) {
	groupsLocal, missing, err := a.st.ListFreshEveGroupsByID(sliceUnique(ids)...)
	if err != nil {
		return nil, err
	}
	groupsRemote, err := fetchObjectsFromAPI(
		missing,
		func(id int32) (esi.GetUniverseGroupsGroupIdOk, *http.Response, error) {
			return a.esiClient.ESI.UniverseApi.GetUniverseGroupsGroupId(context.Background(), id, nil)
		},
		func(x esi.GetUniverseGroupsGroupIdOk) EveGroup {
			return EveGroup{
				CategoryID: x.CategoryId,
				GroupID:    x.GroupId,
				Name:       x.Name,
				Published:  x.Published,
				Timestamp:  now(),
			}
		},
		func(id int32) EveGroup {
			return EveGroup{
				GroupID:   id,
				Name:      nameInvalid,
				Timestamp: now(),
			}
		},
	)
	if len(groupsRemote) > 0 {
		err := a.st.UpdateOrCreateEveGroups(groupsRemote)
		if err != nil {
			return nil, err
		}
	}
	groups := slices.Concat(groupsLocal, groupsRemote)
	return groups, err
}

func sliceUnique[T comparable](s []T) []T {
	m := make(map[T]bool)
	for _, v := range s {
		m[v] = true
	}
	return slices.Collect(maps.Keys(m))
}

func makeLookupMap[T Identifiable](objs []T) map[int32]T {
	m := make(map[int32]T)
	for _, o := range objs {
		m[o.ID()] = o
	}
	return m
}

func fetchObjects[X any, Y Identifiable](ids []int32, fetcherStorage func([]int32) ([]Y, []int32, error), fetcherAPI func(id int32) (X, *http.Response, error), mapper func(x X) Y, invalid func(id int32) Y, storer func([]Y) error) ([]Y, error) {
	objsLocal, missing, err := fetcherStorage(sliceUnique(ids))
	if err != nil {
		return nil, err
	}
	objsRemote := make([]Y, len(missing))
	g := new(errgroup.Group)
	for i, id := range missing {
		g.Go(func() error {
			x, r, err := fetcherAPI(id)
			if err != nil {
				if r != nil && r.StatusCode == http.StatusNotFound {
					objsRemote[i] = invalid(id)
					return nil
				}
				return err
			}
			objsRemote[i] = mapper(x)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	if len(objsRemote) > 0 {
		err := storer(objsRemote)
		if err != nil {
			return nil, err
		}
	}
	objs := slices.Concat(objsLocal, objsRemote)
	return objs, nil
}

func fetchObjectsFromAPI[X any, Y Identifiable](ids []int32, fetcher func(id int32) (X, *http.Response, error), mapper func(x X) Y, invalid func(id int32) Y) ([]Y, error) {
	ids2 := sliceUnique(ids)
	objs := make([]Y, len(ids2))
	g := new(errgroup.Group)
	for i, id := range ids2 {
		g.Go(func() error {
			x, r, err := fetcher(id)
			if err != nil {
				if r != nil && r.StatusCode == http.StatusNotFound {
					objs[i] = invalid(id)
					return nil
				}
				return err
			}
			objs[i] = mapper(x)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return objs, nil
}

func printTableWithSort[T Identifiable](out io.Writer, headers []string, objs []T, makeRow func(T) []any) {
	slices.SortFunc(objs, func(a, b T) int {
		return cmp.Compare(a.ID(), b.ID())
	})
	rows := make([][]any, 0)
	for _, o := range objs {
		rows = append(rows, makeRow(o))
	}
	t := tablewriter.NewTable(out,
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
	t.Header(headers)
	t.Bulk(rows)
	t.Render()
}

func now() time.Time {
	return time.Now().UTC()
}
