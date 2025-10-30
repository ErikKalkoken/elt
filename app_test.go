package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/antihax/goesi"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

func TestApp(t *testing.T) {
	type entity struct {
		ID       int32  `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
	}
	entities := []entity{
		{10000030, "Heimatar", "region"},
		{20000372, "Hed", "constellation"},
		{30002537, "Amamake", "solar_system"},
		{93330670, "Erik Kalkoken", "character"},
		{98267621, "The Congregation", "corporation"},
		{99013305, "RAPID HEAVY ROPERS", "alliance"},
		{60002590, "Amamake VI - Moon 1 - Expert Distribution Warehouse", "staion"},
	}
	entityLookup := make(map[int32]entity)
	for _, o := range entities {
		entityLookup[o.ID] = o
	}
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		`=~^https://esi\.evetech\.net/v\d+/universe/names/`,
		func(req *http.Request) (*http.Response, error) {
			var ids []int32
			if err := json.NewDecoder(req.Body).Decode(&ids); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			var results []entity
			for _, id := range ids {
				r, found := entityLookup[id]
				if !found {
					return httpmock.NewJsonResponse(404, map[string]any{
						"error": "not found",
					})
				}
				results = append(results, r)
			}
			return httpmock.NewJsonResponse(200, results)
		},
	)

	httpmock.RegisterResponder(
		"POST",
		`=~^https://esi\.evetech\.net/v\d+/universe/ids/`,
		func(req *http.Request) (*http.Response, error) {
			var names []string
			if err := json.NewDecoder(req.Body).Decode(&names); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			namesLookup := make(map[string]bool)
			for _, n := range names {
				namesLookup[n] = true
			}
			var matches []entity
			for _, o := range entities {
				if !namesLookup[o.Name] {
					continue
				}
				matches = append(matches, o)
			}
			result := make(map[string][]map[string]any)
			for _, m := range matches {
				c := m.Category + "s"
				result[c] = append(result[c], map[string]any{
					"id":   m.ID,
					"name": m.Name,
				})
			}
			return httpmock.NewJsonResponse(200, result)
		},
	)

	makeObjectEndpoint := func(req *http.Request, data map[int64]map[string]any) (*http.Response, error) {
		id := httpmock.MustGetSubmatchAsInt(req, 1)
		r, found := data[id]
		if !found {
			return httpmock.NewJsonResponse(404, map[string]any{
				"error": "not found",
			})
		}
		return httpmock.NewJsonResponse(200, r)
	}
	httpmock.RegisterResponder(
		"GET",
		`=~^https://esi\.evetech\.net/v\d+/alliances/(\d+)/`,
		func(req *http.Request) (*http.Response, error) {
			data := map[int64]map[string]any{
				99013305: {
					"creator_corporation_id":  98699354,
					"creator_id":              2119493499,
					"date_founded":            "2024-06-02T17:24:57Z",
					"executor_corporation_id": 98699354,
					"name":                    "RAPID HEAVY ROPERS",
					"ticker":                  "ROPE",
				},
			}
			return makeObjectEndpoint(req, data)
		},
	)
	httpmock.RegisterResponder(
		"GET",
		`=~^https://esi\.evetech\.net/v\d+/corporations/(\d+)/`,
		func(req *http.Request) (*http.Response, error) {
			data := map[int64]map[string]any{
				98267621: {
					"alliance_id":     99013305,
					"ceo_id":          1559150123,
					"creator_id":      1559150123,
					"date_founded":    "2013-11-26T21:41:51Z",
					"description":     "<font size=\"14\" color=\"#bfffffff\"></font><font size=\"12\" color=\"#bfffffff\">There is no hunting like the hunting of man, and those who have hunted armed men long enough and liked it, never care for anything else thereafter.<br>-Ernest Hemingway<br><br>[19:48:38] raspin manin forter &gt; baltrom ur a cry baby<br><br>[20:29:29] Paquito &gt; classic coward move<br><br><br>Recruitment : </font><font size=\"12\" color=\"#ff00ff00\">OPEN<br><br><b>--&gt;  </font><font size=\"12\" color=\"#ff6868e1\"><a href=\"joinChannel:-72281221//None//None\">Rabis Pub</a></font><font size=\"12\" color=\"#ff00ff00\"> <br><br>Corporation CEO - </font><font size=\"12\" color=\"#ffd98d00\"><a href=\"showinfo:1385//1559150123\">Baltrom</a></font><font size=\"12\" color=\"#ff00ff00\"> <br>Corporation Office Manager - </font><font size=\"12\" color=\"#ffd98d00\"><loc><a href=\"showinfo:1377//1007617072\">Benzmann</a></loc><br></font><font size=\"12\" color=\"#ff00ff00\">Corporation Head Diplomat -  </font><font size=\"12\" color=\"#ffd98d00\"><a href=\"showinfo:1374//2115450815\">D43DLY D43DLY</a></font><font size=\"12\" color=\"#ff00ff00\">  <br>Corporation Junior Diplomat -</b> </font><font size=\"12\" color=\"#ffd98d00\"><a href=\"showinfo:1377//2113096754\">Titan Ofc</a></font><font size=\"12\" color=\"#ff00ff00\"> <br><b>Deputy Assistant Diplomat -  </font><font size=\"12\" color=\"#ffd98d00\"><a href=\"showinfo:1375//95767597\">Nyth Hinken</a></font><font size=\"12\" color=\"#ff00ff00\"> <br><br>Under 14s Pilot Liaison -  </font><font size=\"12\" color=\"#ffd98d00\"><a href=\"showinfo:1373//666628406\">Ashterothi</a></font><font size=\"12\" color=\"#ff00ff00\"> <br><br></font><font size=\"13\" color=\"#ffff00ff\"><u>ACTIVE</u> Campaign Commander -</font><font size=\"12\" color=\"#ffff00ff\"> </font><font size=\"12\" color=\"#ffd98d00\"><a href=\"showinfo:1379//2112874265\">Blights Wretch</a></font><font size=\"12\" color=\"#ffff00ff\"> </b></font>",
					"home_station_id": 60015111,
					"member_count":    50,
					"name":            "The Congregation",
					"shares":          1000,
					"tax_rate":        0.05000000074505806,
					"ticker":          "RABIS",
					"url":             "https://www.rabis.space/home",
					"war_eligible":    true,
				},
			}
			return makeObjectEndpoint(req, data)
		},
	)
	httpmock.RegisterResponder(
		"GET",
		`=~^https://esi\.evetech\.net/v\d+/characters/(\d+)/`,
		func(req *http.Request) (*http.Response, error) {
			data := map[int64]map[string]any{
				93330670: {
					"alliance_id":     99013305,
					"birthday":        "2013-05-12T00:19:09Z",
					"bloodline_id":    1,
					"corporation_id":  98267621,
					"description":     "<font size=\"13\" color=\"#bfffffff\">These days I mostly \"play EVE\" by working on my EVE related apps. Here are a few highlights:<br><br>- </font><font size=\"13\" color=\"#ffffe400\"><loc><a href=\"https://github.com/ErikKalkoken/evebuddy\">EVE Buddy</a></loc></font><font size=\"13\" color=\"#bfffffff\"> - A desktop companion app for Windows, Linux and macOS.<br>- </font><font size=\"13\" color=\"#ffffe400\"><loc><a href=\"https://gitlab.com/ErikKalkoken/aa-structures\">Structures</a></loc></font><font size=\"13\" color=\"#bfffffff\"> - An app for managing Eve Online structures with Alliance Auth.<br>- </font><font size=\"13\" color=\"#ffffe400\"><loc><a href=\"https://gitlab.com/ErikKalkoken/aa-memberaudit\">Member Audit</a></loc></font><font size=\"13\" color=\"#bfffffff\"> - An Alliance Auth app that provides full access to Eve characters and related reports for auditing, vetting and monitoring.<br><br>I also have a </font><font size=\"13\" color=\"#ffffe400\"><loc><a href=\"https://erikkalkoken.gitlab.io/blog/\">blog</a></loc></font><font size=\"13\" color=\"#bfffffff\"> where I sometimes write about Alliance Auth and programming related topics.</font>",
					"gender":          "male",
					"name":            "Erik Kalkoken",
					"race_id":         1,
					"security_status": -10,
					"title":           "https://youtu.be/OplObfGNiJ4?t=5",
				},
			}
			return makeObjectEndpoint(req, data)
		},
	)
	httpmock.RegisterResponder(
		"GET",
		`=~^https://esi\.evetech\.net/v\d+/universe/constellations/(\d+)/`,
		func(req *http.Request) (*http.Response, error) {
			data := map[int64]map[string]any{
				20000372: {
					"constellation_id": 20000372,
					"name":             "Hed",
					"position": map[string]any{
						"x": -128269338426337400,
						"y": 38212719069804984,
						"z": 7556108809294752,
					},
					"region_id": 10000030,
					"systems": []int{
						30002537,
						30002538,
						30002539,
						30002540,
						30002541,
						30002542,
					},
				},
			}
			return makeObjectEndpoint(req, data)
		},
	)
	httpmock.RegisterResponder(
		"GET",
		`=~^https://esi\.evetech\.net/v\d+/universe/regions/(\d+)/`,
		func(req *http.Request) (*http.Response, error) {
			data := map[int64]map[string]any{
				10000030: {
					"constellations": []int{
						20000367,
						20000368,
						20000369,
						20000370,
						20000371,
						20000372,
						20000373,
						20000374,
						20000375,
						20000376,
						20000377,
						20000378,
					},
					"description": "\"Never Again\"",
					"name":        "Heimatar",
					"region_id":   10000030,
				},
			}
			return makeObjectEndpoint(req, data)
		},
	)
	httpmock.RegisterResponder(
		"GET",
		`=~^https://esi\.evetech\.net/v\d+/universe/systems/(\d+)/`,
		func(req *http.Request) (*http.Response, error) {
			data := map[int64]map[string]any{
				30002537: {
					"constellation_id": 20000372,
					"name":             "Amamake",
					"planets": []map[string]any{
						{
							"planet_id": 40161463,
						},
						{
							"moons": []int{
								40161465,
								40161466,
							},
							"planet_id": 40161464,
						},
						{
							"asteroid_belts": []int{
								40161468,
							},
							"planet_id": 40161467,
						},
						{
							"asteroid_belts": []int{
								40161470,
							},
							"moons": []int{
								40161471,
							},
							"planet_id": 40161469,
						},
						{
							"asteroid_belts": []int{
								40161473,
								40161475,
							},
							"moons": []int{
								40161474,
							},
							"planet_id": 40161472,
						},
						{
							"asteroid_belts": []int{
								40161478,
								40161490,
								40161494,
								40161495,
								40161497,
							},
							"moons": []int{
								40161477,
								40161479,
								40161480,
								40161481,
								40161482,
								40161483,
								40161484,
								40161485,
								40161486,
								40161487,
								40161488,
								40161489,
								40161491,
								40161492,
								40161493,
								40161496,
								40161498,
							},
							"planet_id": 40161476,
						},
					},
					"position": map[string]any{
						"x": -124292266288000000,
						"y": 44194364193700000,
						"z": 6110392433590000,
					},
					"security_class":  "E",
					"security_status": 0.43876123428344727,
					"star_id":         40161462,
					"stargates": []int{
						50004548,
						50004549,
						50004550,
						50004551,
						50004552,
						50013705,
					},
					"stations": []int{
						60002590,
						60002596,
						60002599,
						60004597,
						60004603,
						60004816,
						60004819,
						60004822,
						60004831,
						60005035,
						60005038,
						60007333,
						60007339,
						60007342,
						60007345,
						60007684,
						60007687,
						60007690,
						60014827,
						60015175,
					},
					"system_id": 30002537,
				},
			}
			return makeObjectEndpoint(req, data)
		},
	)
	httpmock.RegisterResponder(
		"GET",
		`=~^https://esi\.evetech\.net/v\d+/universe/systems/(\d+)/`,
		func(req *http.Request) (*http.Response, error) {
			data := map[int64]map[string]any{
				60002590: {
					"max_dockable_ship_volume": 50000000,
					"name":                     "Amamake VI - Moon 1 - Expert Distribution Warehouse",
					"office_rental_cost":       10000,
					"owner":                    98267621,
					"position": map[string]any{
						"x": -442534010880,
						"y": -58789109760,
						"z": 1018829660160,
					},
					"race_id":                    1,
					"reprocessing_efficiency":    0.5,
					"reprocessing_stations_take": 0.05,
					"services": []string{
						"bounty-missions",
						"courier-missions",
						"reprocessing-plant",
						"market",
						"repair-facilities",
						"fitting",
						"news",
						"insurance",
						"docking",
						"office-rental",
						"loyalty-point-store",
						"navy-offices",
					},
					"station_id": 60002590,
					"system_id":  30002537,
					"type_id":    1531,
				},
			}
			return makeObjectEndpoint(req, data)
		},
	)

	p := filepath.Join(t.TempDir(), "elt.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := NewStorage(db)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	esiClient := goesi.NewAPIClient(nil, "")
	for _, o := range entities {
		t.Run(fmt.Sprintf("can resolve %s ID", o.Category), func(t *testing.T) {
			st.Clear()
			var buf bytes.Buffer
			a := NewApp(esiClient, st, &buf, 0)
			err := a.Run([]string{fmt.Sprint(o.ID)}, false)
			if !assert.NoError(t, err) {
				t.Fatal(err)
			}
			got := buf.String()
			assert.Contains(t, got, EveEntityCategory(o.Category).Display())
			assert.Contains(t, got, fmt.Sprint(o.ID))
			assert.Contains(t, got, o.Name)
			assert.NotContains(t, got, "INVALID")
		})
	}
}
