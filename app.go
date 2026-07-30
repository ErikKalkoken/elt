package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"time"

	"github.com/ErikKalkoken/eveauth"
	"github.com/ErikKalkoken/go-set"
	"github.com/fnt-eve/goesi-openapi"
	"github.com/fnt-eve/goesi-openapi/esi"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

const (
	nameInvalid = "INVALID"
)

type AuthClient interface {
	Authorize(ctx context.Context, scopes []string) (*eveauth.Token, error)
	RefreshToken(ctx context.Context, token *eveauth.Token) error
}

type result struct {
	category EveEntityCategory
	count    int
	table    *tablewriter.Table
}

type App struct {
	// Whether to show the spinner
	SpinnerDisabled bool

	// When specified limit the results to this category
	EntityCategory EveEntityCategory

	// Max returned results.
	MaxResults int

	// Max width of the terminal in characters.
	MaxWidth int

	authClient AuthClient
	esiClient  *esi.APIClient
	out        io.Writer
	st         *Storage
}

func NewApp(authClient AuthClient, esiClient *esi.APIClient, st *Storage, out io.Writer) App {
	a := App{
		authClient: authClient,
		esiClient:  esiClient,
		out:        out,
		st:         st,
	}
	return a
}

func (a App) Authorize() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		token *eveauth.Token
		err   error
	}
	resultCh := make(chan result)
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt)
	var bar *progressbar.ProgressBar
	if !a.SpinnerDisabled {
		bar = progressbar.NewOptions(-1,
			progressbar.OptionSpinnerType(14), // choose spinner style (0–39)
			progressbar.OptionSetDescription("Starting authorization in browser. CTRL-C to abort"),
			progressbar.OptionSetElapsedTime(false),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionSetWriter(a.out),
			progressbar.OptionClearOnFinish(),
		)
	}
	go func() {
		token, err := a.authClient.Authorize(
			ctx,
			[]string{"esi-search.search_structures.v1"},
		)
		resultCh <- result{token, err}
	}()
	var r result
	select {
	case r = <-resultCh:
	case <-stopCh:
		cancel()
		r = <-resultCh
	}
	if bar != nil {
		bar.Finish()
	}
	if errors.Is(r.err, eveauth.ErrAborted) {
		fmt.Fprintln(a.out, "Authorization flow has been canceled")
		return nil
	}
	if r.err != nil {
		return r.err
	}
	if err := a.st.UpdateOrCreateEveToken(EveToken{
		AccessToken:   r.token.AccessToken,
		CharacterID:   r.token.CharacterID,
		CharacterName: r.token.CharacterName,
		ExpiresAt:     r.token.ExpiresAt,
		RefreshToken:  r.token.RefreshToken,
		Scopes:        r.token.Scopes,
		TokenType:     r.token.TokenType,
	}); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%s has been authorized with character %s\n", appName, r.token.CharacterName)
	return nil
}

// Lookup is the main entry point.
func (a App) Lookup(args []string) error {
	// Parse args
	var (
		ids     set.Set[int64]
		invalid set.Set[int]
		names   set.Set[string]
	)
	for _, arg := range args {
		id, err := strconv.Atoi(arg)
		if err != nil {
			names.Add(arg)
		} else {
			id32 := int64(id)
			if int(id32) != id || id == 0 {
				invalid.Add(id)
				continue
			}
			ids.Add(id32)
		}
	}
	if invalid.Size() > 0 {
		fmt.Fprintf(a.out, "Ignoring invalid IDs: %v\n", invalid)
	}

	count := ids.Size() + names.Size()
	if count == 0 {
		return fmt.Errorf("no suitable input to process")
	}

	// Resolve ids and names
	var bar *progressbar.ProgressBar
	if !a.SpinnerDisabled {
		bar = progressbar.NewOptions(-1,
			progressbar.OptionSpinnerType(14), // choose spinner style (0–39)
			progressbar.OptionSetDescription(fmt.Sprintf("Resolving %d IDs/names ...", count)),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionSetWriter(a.out),
			progressbar.OptionClearOnFinish(),
		)
	}
	g := new(errgroup.Group)
	var entities1, entities2 []EveEntity
	if ids.Size() > 0 {
		g.Go(func() error {
			oo, err := a.resolveIDs(ids)
			if err != nil {
				return err
			}
			entities1 = oo
			return nil
		})
	}
	if names.Size() > 0 {
		g.Go(func() error {
			oo, err := a.resolveNames(names)
			if err != nil {
				return err
			}
			entities2 = oo
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	entities := slices.Concat(entities1, entities2)
	slog.Info("resolved entities from input values", "count", len(entities))
	slices.SortFunc(entities, func(a, b EveEntity) int {
		return cmp.Compare(a.EntityID, b.EntityID)
	})
	totalCount := len(entities)
	if a.MaxResults != 0 && totalCount > a.MaxResults {
		entities = entities[:a.MaxResults]
	}
	return a.compileResults(entities, bar, totalCount)
}

func (a App) Search(search string) error {
	if search == "" {
		return fmt.Errorf("please provide a non-empty search string")
	}
	tok, err := a.st.GetEveToken()
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("you must authorize elt before you can use search")
	}
	var bar *progressbar.ProgressBar
	if !a.SpinnerDisabled {
		bar = progressbar.NewOptions(-1,
			progressbar.OptionSpinnerType(14), // choose spinner style (0–39)
			progressbar.OptionSetDescription("Searching..."),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionSetWriter(a.out),
			progressbar.OptionClearOnFinish(),
		)
	}
	if time.Until(tok.ExpiresAt) < 30*time.Second {
		bar.Describe("Refreshing token...")
		tok2 := &eveauth.Token{
			AccessToken:   tok.AccessToken,
			CharacterID:   tok.CharacterID,
			CharacterName: tok.CharacterName,
			ExpiresAt:     tok.ExpiresAt,
			RefreshToken:  tok.RefreshToken,
			Scopes:        tok.Scopes,
			TokenType:     tok.TokenType,
		}
		err := a.authClient.RefreshToken(context.Background(), tok2)
		if err != nil {
			return err
		}
		tok.AccessToken = tok2.AccessToken
		tok.RefreshToken = tok2.RefreshToken
		tok.ExpiresAt = tok2.ExpiresAt
		if err := a.st.UpdateOrCreateEveToken(tok); err != nil {
			return err
		}
	}
	if bar != nil {
		bar.Describe(fmt.Sprintf("Searching for %s...", search))
	}
	var categories []string
	if a.EntityCategory != CategoryUndefined {
		categories = []string{string(a.EntityCategory)}
	} else {
		categories = []string{
			"agent",
			"alliance",
			"character",
			"constellation",
			"corporation",
			"faction",
			"inventory_type",
			"region",
			"solar_system",
			"station",
		}
	}
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: tok.AccessToken,
	})
	ctx := context.WithValue(context.Background(), goesi.ContextOAuth2, tokenSource)
	x, _, err := a.esiClient.SearchAPI.GetCharactersCharacterIdSearch(ctx, int64(tok.CharacterID)).Search(search).Categories(categories).Execute()
	if err != nil {
		return err
	}
	ids := slices.Concat(
		x.Agent,
		x.Alliance,
		x.Character,
		x.Corporation,
		x.Constellation,
		x.Faction,
		x.InventoryType,
		x.SolarSystem,
		x.Station,
		x.Region,
	)
	slices.Sort(ids)
	totalCount := len(ids)
	if a.MaxResults != 0 && totalCount > a.MaxResults {
		ids = ids[:a.MaxResults]
	}
	oo, err := a.resolveIDs(set.Of(ids...))
	if err != nil {
		return err
	}
	if err := a.compileResults(oo, bar, totalCount); err != nil {
		return err
	}
	return nil
}

func (a App) compileResults(entities []EveEntity, bar *progressbar.ProgressBar, totalCount int) error {
	category2IDs := make(map[EveEntityCategory]set.Set[int64])
	for _, e := range entities {
		if a.EntityCategory != CategoryUndefined && a.EntityCategory != e.Category {
			continue
		}
		x := category2IDs[e.Category]
		x.Add(e.ID())
		category2IDs[e.Category] = x
	}
	results := make([]result, len(category2IDs))
	if len(results) == 0 {
		if bar != nil {
			bar.Finish()
		}
		fmt.Fprintln(a.out, "Nothing found")
		return nil
	}
	if bar != nil {
		bar.Describe(fmt.Sprintf("Compiling %d results...", len(entities)))
	}
	g2 := new(errgroup.Group)
	for i, c := range slices.Sorted(maps.Keys(category2IDs)) {
		g2.Go(func() error {
			ids := category2IDs[c]
			switch c {
			case CategoryAgent:
				t, err := a.buildCharacterTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
			case CategoryAlliance:
				t, err := a.buildAllianceTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
			case CategoryCharacter:
				t, err := a.buildCharacterTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
			case CategoryConstellation:
				t, err := a.buildConstellationTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
			case CategoryCorporation:
				t, err := a.buildCorporationTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
			case CategoryFaction:
				t, err := a.buildFactionTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
			case CategoryInventoryType:
				t, err := a.buildTypeTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
			case CategoryRegion:
				t, err := a.buildRegionTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
			case CategorySolarSystem:
				t, err := a.buildSolarSystemTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
			case CategoryStation:
				t, err := a.buildStationTable(ids)
				if err != nil {
					return err
				}
				results[i] = result{category: c, count: ids.Size(), table: t}
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
				results[i] = result{category: c, count: len(entities2), table: t}
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
				results[i] = result{category: c, count: 0, table: t}
			}
			slog.Info("Resolved objects", "category", c, "count", ids.Size())
			return nil
		})
	}
	if err := g2.Wait(); err != nil {
		return err
	}

	if bar != nil {
		bar.Finish()
	}

	// Print results
	currentCount := len(entities)
	if totalCount > currentCount {
		fmt.Fprintf(a.out, "Found %d result(s) (truncated from %d total):\n", currentCount, totalCount)
	} else {
		fmt.Fprintf(a.out, "Found %d result(s):\n", totalCount)
	}
	for _, r := range results {
		if r.table == nil {
			continue
		}
		fmt.Fprintf(a.out, "%s (%d):\n", r.category.Display(), r.count)
		err := r.table.Render()
		if err != nil {
			return err
		}
	}
	return nil
}

func (a App) resolveIDs(ids set.Set[int64]) ([]EveEntity, error) {
	entities1, unknownIDs, err := a.st.ListFreshEveEntityByID(ids)
	if err != nil {
		return nil, err
	}
	entities2, err := resolveIDsFromAPI(a.esiClient, unknownIDs)
	if err != nil {
		return nil, err
	}
	entities3 := slices.DeleteFunc(slices.Clone(entities2), func(o EveEntity) bool {
		return o.ID() == 0
	})
	if err := a.st.UpdateOrCreateEveEntity(entities3); err != nil {
		return nil, err
	}
	m := make(map[int64]EveEntity)
	for _, e := range slices.Concat(entities1, entities2) {
		m[e.EntityID] = e
	}
	entities := make([]EveEntity, 0)
	for id := range ids.All() {
		entities = append(entities, m[id])
	}
	return entities, nil
}

func resolveIDsFromAPI(esiClient *esi.APIClient, ids set.Set[int64]) ([]EveEntity, error) {
	entities := make([]EveEntity, 0)
	for idsChunk := range slices.Chunk(slices.Collect(ids.All()), 1000) {
		oo, err := resolveIDsFromAPI2(esiClient, idsChunk)
		if err != nil {
			return nil, err
		}
		entities = slices.Concat(entities, oo)
	}
	return entities, nil
}

func resolveIDsFromAPI2(esiClient *esi.APIClient, ids []int64) ([]EveEntity, error) {
	if len(ids) == 0 {
		return []EveEntity{}, nil
	}
	entities, err := resolveIDsFromAPI3(esiClient, ids)
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
			entities, err := resolveIDsFromAPI2(esiClient, ids[:n/2])
			if err != nil {
				return err
			}
			it1 = entities
			return nil
		})
		g.Go(func() error {
			entities, err := resolveIDsFromAPI2(esiClient, ids[n/2:])
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

func resolveIDsFromAPI3(esiClient *esi.APIClient, ids []int64) ([]EveEntity, error) {
	data, r, err := esiClient.UniverseAPI.PostUniverseNames(context.Background()).RequestBody(ids).Execute()
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

func (a App) resolveNames(names set.Set[string]) ([]EveEntity, error) {
	if names.Size() == 0 {
		return []EveEntity{}, nil
	}
	data, r, err := a.esiClient.UniverseAPI.PostUniverseIds(context.Background()).RequestBody(slices.Collect(names.All())).Execute()
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error: %s", r.Status)
	}
	matches := make(map[string]bool)
	for n := range names.All() {
		matches[n] = true
	}
	found := make(map[string]bool)
	entities := make([]EveEntity, 0)
	addEntity := func(id int64, name string, category EveEntityCategory) {
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
		addEntity(o.GetId(), o.GetName(), CategoryAgent)
	}
	for _, o := range data.Alliances {
		addEntity(o.GetId(), o.GetName(), CategoryAlliance)
	}
	for _, o := range data.Characters {
		addEntity(o.GetId(), o.GetName(), CategoryCharacter)
	}
	for _, o := range data.Constellations {
		addEntity(o.GetId(), o.GetName(), CategoryConstellation)
	}
	for _, o := range data.Corporations {
		addEntity(o.GetId(), o.GetName(), CategoryCorporation)
	}
	for _, o := range data.Factions {
		addEntity(o.GetId(), o.GetName(), CategoryFaction)
	}
	for _, o := range data.InventoryTypes {
		addEntity(o.GetId(), o.GetName(), CategoryInventoryType)
	}
	for _, o := range data.Regions {
		addEntity(o.GetId(), o.GetName(), CategoryRegion)
	}
	for _, o := range data.Stations {
		addEntity(o.GetId(), o.GetName(), CategoryStation)
	}
	for _, o := range data.Systems {
		addEntity(o.GetId(), o.GetName(), CategorySolarSystem)
	}
	for n := range names.All() {
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

func (a App) buildCharacterTable(ids set.Set[int64]) (*tablewriter.Table, error) {
	characters, err := a.fetchCharacters(ids)
	if err != nil {
		return nil, err
	}
	var entityIDs set.Set[int64]
	for _, o := range characters {
		entityIDs.Add(o.CorporationID)
		if o.AllianceID != 0 {
			entityIDs.Add(o.AllianceID)
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

func (a App) fetchCharacters(ids set.Set[int64]) ([]EveCharacter, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveCharacterByID,
		func(id int64) (*esi.CharactersDetail, *http.Response, error) {
			return a.esiClient.CharacterAPI.GetCharactersDetail(context.Background(), id).Execute()
		},
		func(id int64, x *esi.CharactersDetail) EveCharacter {
			return EveCharacter{
				AllianceID:    x.GetAllianceId(),
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

func (a App) buildCorporationTable(ids set.Set[int64]) (*tablewriter.Table, error) {
	corporations, err := a.fetchCorporations(ids)
	if err != nil {
		return nil, err
	}
	var entityIDs set.Set[int64]
	for _, o := range corporations {
		if o.AllianceID != 0 {
			entityIDs.Add(o.AllianceID)
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

func (a App) fetchCorporations(ids set.Set[int64]) ([]EveCorporation, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveCorporationByID,
		func(id int64) (*esi.CorporationsDetail, *http.Response, error) {
			return a.esiClient.CorporationAPI.GetCorporationsCorporationId(context.Background(), id).Execute()
		},
		func(id int64, x *esi.CorporationsDetail) EveCorporation {
			return EveCorporation{
				AllianceID:    x.GetAllianceId(),
				CeoID:         x.GetCeoId(),
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

func (a App) buildAllianceTable(ids set.Set[int64]) (*tablewriter.Table, error) {
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

func (a App) fetchAlliance(ids set.Set[int64]) ([]EveAlliance, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveAllianceByID,
		func(id int64) (*esi.AllianceDetail, *http.Response, error) {
			return a.esiClient.AllianceAPI.GetAlliancesAllianceId(context.Background(), id).Execute()
		},
		func(id int64, x *esi.AllianceDetail) EveAlliance {
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

func (a App) buildFactionTable(ids set.Set[int64]) (*tablewriter.Table, error) {
	factions, err := a.fetchFactions(ids)
	if err != nil {
		return nil, err
	}
	var entityIDs set.Set[int64]
	for _, o := range factions {
		if o.CorporationID != 0 {
			entityIDs.Add(o.CorporationID)
		}
		if o.MilitiaCorporationID != 0 {
			entityIDs.Add(o.MilitiaCorporationID)
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

func (a App) fetchFactions(ids set.Set[int64]) ([]EveFaction, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveFactionByID,
		func(id int64) ([]esi.UniverseFactionsGetInner, *http.Response, error) {
			return a.esiClient.UniverseAPI.GetUniverseFactions(context.Background()).Execute()
		},
		func(id int64, xx []esi.UniverseFactionsGetInner) EveFaction {
			for _, x := range xx {
				if x.FactionId != id {
					continue
				}
				return EveFaction{
					FactionID:            id,
					CorporationID:        x.GetCorporationId(),
					MilitiaCorporationID: x.GetMilitiaCorporationId(),
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

func (a App) buildStationTable(ids set.Set[int64]) (*tablewriter.Table, error) {
	stations, err := a.fetchStations(ids)
	if err != nil {
		return nil, err
	}
	var entityIDs set.Set[int64]
	for _, et := range stations {
		entityIDs.Add(et.OwnerID, et.SolarSystemID, et.TypeID)
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

func (a App) fetchStations(ids set.Set[int64]) ([]EveStation, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveStationByID,
		func(id int64) (*esi.UniverseStationsStationIdGet, *http.Response, error) {
			return a.esiClient.UniverseAPI.GetUniverseStationsStationId(context.Background(), id).Execute()
		},
		func(id int64, x *esi.UniverseStationsStationIdGet) EveStation {
			return EveStation{
				Name:          x.Name,
				OwnerID:       x.GetOwner(),
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

func (a App) buildTypeTable(ids set.Set[int64]) (*tablewriter.Table, error) {
	types, err := a.fetchTypes(ids)
	if err != nil {
		return nil, err
	}
	var groupIDs set.Set[int64]
	for _, et := range types {
		groupIDs.Add(et.GroupID)
	}
	groups, err := a.fetchGroups(groupIDs)
	if err != nil {
		return nil, err
	}
	groupLookup := makeLookupMap(groups)
	var categoryIDs set.Set[int64]
	for _, eg := range groups {
		categoryIDs.Add(eg.CategoryID)
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

func (a App) fetchTypes(ids set.Set[int64]) ([]EveType, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveTypeByID,
		func(id int64) (*esi.UniverseTypesTypeIdGet, *http.Response, error) {
			return a.esiClient.UniverseAPI.GetUniverseTypesTypeId(context.Background(), id).Execute()
		},
		func(id int64, x *esi.UniverseTypesTypeIdGet) EveType {
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

func (a App) fetchCategories(ids set.Set[int64]) ([]EveCategory, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveCategoryByID,
		func(id int64) (*esi.UniverseCategoriesCategoryIdGet, *http.Response, error) {
			return a.esiClient.UniverseAPI.GetUniverseCategoriesCategoryId(context.Background(), id).Execute()
		},
		func(id int64, x *esi.UniverseCategoriesCategoryIdGet) EveCategory {
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

func (a App) fetchGroups(ids set.Set[int64]) ([]EveGroup, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveGroupByID,
		func(id int64) (*esi.UniverseGroupsGroupIdGet, *http.Response, error) {
			return a.esiClient.UniverseAPI.GetUniverseGroupsGroupId(context.Background(), id).Execute()
		},
		func(id int64, x *esi.UniverseGroupsGroupIdGet) EveGroup {
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

func (a App) buildSolarSystemTable(ids set.Set[int64]) (*tablewriter.Table, error) {
	types, err := a.fetchSolarSystems(ids)
	if err != nil {
		return nil, err
	}
	var constellationIDs set.Set[int64]
	for _, o := range types {
		constellationIDs.Add(o.ConstellationID)
	}
	constellations, err := a.fetchConstellations(constellationIDs)
	if err != nil {
		return nil, err
	}
	constellationLookup := makeLookupMap(constellations)
	var regionIDs set.Set[int64]
	for _, o := range constellations {
		regionIDs.Add(o.RegionID)
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

func (a App) fetchSolarSystems(ids set.Set[int64]) ([]EveSolarSystem, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveSolarSystemByID,
		func(id int64) (*esi.UniverseSystemsSystemIdGet, *http.Response, error) {
			return a.esiClient.UniverseAPI.GetUniverseSystemsSystemId(context.Background(), id).Execute()
		},
		func(id int64, x *esi.UniverseSystemsSystemIdGet) EveSolarSystem {
			return EveSolarSystem{
				ConstellationID: x.ConstellationId,
				Name:            x.Name,
				Security:        x.GetSecurityStatus(),
				SolarSystemID:   id,
				Timestamp:       now(),
			}
		},
		a.st.UpdateOrCreateEveSolarSystem,
	)
	return oo, err
}

func (a App) buildConstellationTable(ids set.Set[int64]) (*tablewriter.Table, error) {
	constellations, err := a.fetchConstellations(ids)
	if err != nil {
		return nil, err
	}
	var regionIDs set.Set[int64]
	for _, o := range constellations {
		regionIDs.Add(o.RegionID)
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

func (a App) fetchConstellations(ids set.Set[int64]) ([]EveConstellation, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveConstellationByID,
		func(id int64) (*esi.UniverseConstellationsConstellationIdGet, *http.Response, error) {
			return a.esiClient.UniverseAPI.GetUniverseConstellationsConstellationId(context.Background(), id).Execute()
		},
		func(id int64, x *esi.UniverseConstellationsConstellationIdGet) EveConstellation {
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

func (a App) buildRegionTable(ids set.Set[int64]) (*tablewriter.Table, error) {
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

func (a App) fetchRegions(ids set.Set[int64]) ([]EveRegion, error) {
	oo, _, err := fetchObjects(
		ids,
		a.st.ListFreshEveRegionByID,
		func(id int64) (*esi.UniverseRegionsRegionIdGet, *http.Response, error) {
			return a.esiClient.UniverseAPI.GetUniverseRegionsRegionId(context.Background(), id).Execute()
		},
		func(id int64, x *esi.UniverseRegionsRegionIdGet) EveRegion {
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

func idOrEmpty(id int64) string {
	if id == 0 {
		return ""
	}
	return strconv.Itoa(int(id))
}

func makeLookupMap[T EveObject](objs []T) map[int64]T {
	m := make(map[int64]T)
	for _, o := range objs {
		m[o.ID()] = o
	}
	return m
}

// fetchObjects fetches and returns eve objects for the given ids.
// It returns objects from storage when found or otherwise fetches them from the API.
// It also returns a slice of invalid IDs for objects which could not be found.
func fetchObjects[X any, Y EveObject](ids set.Set[int64], fetcherStorage func(set.Set[int64]) ([]Y, set.Set[int64], error), fetcherAPI func(id int64) (X, *http.Response, error), mapper func(id int64, x X) Y, storer func([]Y) error) ([]Y, set.Set[int64], error) {
	wrapErr := func(err error) error {
		var z Y
		return fmt.Errorf("fetch objects %T: %v: %w", z, ids, err)
	}
	objsLocal, missing, err := fetcherStorage(ids)
	if err != nil {
		return nil, set.Set[int64]{}, wrapErr(err)
	}
	objsRemote := make([]Y, missing.Size())
	invalidIDs := make([]int64, missing.Size())
	g := new(errgroup.Group)
	for i, id := range slices.Collect(missing.All()) {
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
		return nil, set.Set[int64]{}, wrapErr(err)
	}
	if len(objsRemote) > 0 {
		oo := slices.DeleteFunc(objsRemote, func(x Y) bool {
			return x.ID() == 0
		})
		err := storer(oo)
		if err != nil {
			return nil, set.Set[int64]{}, wrapErr(err)
		}
	}
	invalid2 := set.Of(invalidIDs...)
	invalid2.DeleteFunc(func(x int64) bool {
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
