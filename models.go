package main

import (
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	day                   = 24 * time.Hour
	week                  = 24 * 7 * time.Hour
	npcCorporationIDBegin = 1_000_000
	npcCorporationIDEnd   = 2_000_000
	npcCharacterIDBegin   = 3_000_000
	npcCharacterIDEnd     = 4_000_000
)

type EveEntityCategory string

// Supported categories of EveEntity
const (
	CategoryUndefined     EveEntityCategory = ""
	CategoryAgent         EveEntityCategory = "agent"
	CategoryAlliance      EveEntityCategory = "alliance"
	CategoryCharacter     EveEntityCategory = "character"
	CategoryConstellation EveEntityCategory = "constellation"
	CategoryCorporation   EveEntityCategory = "corporation"
	CategoryFaction       EveEntityCategory = "faction"
	CategoryInventoryType EveEntityCategory = "inventory_type"
	CategoryRegion        EveEntityCategory = "region"
	CategorySolarSystem   EveEntityCategory = "solar_system"
	CategoryStation       EveEntityCategory = "station"
	CategoryInvalid       EveEntityCategory = "invalid"
	CategoryUnknown       EveEntityCategory = "unknown" // CategoryUnknown represents a new or changed category
)

func (c EveEntityCategory) Display() string {
	switch c {
	case CategoryInvalid:
		return "INVALID"
	case CategoryUnknown:
		return "?"
	}
	c2 := strings.ReplaceAll(string(c), "_", " ")
	return cases.Title(language.English).String(c2)
}

type EveEntity struct {
	Category  EveEntityCategory `json:"category"`
	EntityID  int32             `json:"entity_id"`
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
}

func (o EveEntity) ID() int32 {
	return o.EntityID
}

func (o EveEntity) IsStale() bool {
	switch o.Category {
	case CategoryInventoryType:
		return o.Timestamp.Before(time.Now().UTC().Add(-week))
	default:
		return o.Timestamp.Before(time.Now().UTC().Add(-day))
	}
}

func (o EveEntity) IsValid() bool {
	return o.ID() != 0 && o.Category != CategoryUndefined
}

type EveAlliance struct {
	AllianceID int32     `json:"alliance_id"`
	Name       string    `json:"name"`
	Ticker     string    `json:"ticker"`
	Timestamp  time.Time `json:"timestamp"`
}

func (o EveAlliance) ID() int32 {
	return o.AllianceID
}

func (o EveAlliance) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-day))
}

func (o EveAlliance) IsValid() bool {
	return o.ID() != 0
}

type EveCategory struct {
	CategoryID int32     `json:"category_id"`
	Name       string    `json:"name"`
	Published  bool      `json:"published"`
	Timestamp  time.Time `json:"timestamp"`
}

func (o EveCategory) ID() int32 {
	return o.CategoryID
}

func (o EveCategory) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-week))
}

func (o EveCategory) IsValid() bool {
	return o.ID() != 0
}

type EveCharacter struct {
	AllianceID    int32     `json:"alliance_id"`
	CharacterID   int32     `json:"character_id"`
	CorporationID int32     `json:"corporation_id"`
	Name          string    `json:"name"`
	Timestamp     time.Time `json:"timestamp"`
}

func (o EveCharacter) ID() int32 {
	return o.CharacterID
}

func (o EveCharacter) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-day))
}

func (o EveCharacter) IsNPC() bool {
	if o.CharacterID >= npcCharacterIDBegin && o.CharacterID < npcCharacterIDEnd {
		return true
	}
	return false
}

func (o EveCharacter) IsValid() bool {
	return o.ID() != 0
}

type EveConstellation struct {
	ConstellationID int32     `json:"constellation_id"`
	Name            string    `json:"name"`
	RegionID        int32     `json:"region_id"`
	Timestamp       time.Time `json:"timestamp"`
}

func (o EveConstellation) ID() int32 {
	return o.ConstellationID
}

func (o EveConstellation) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-week))
}

func (o EveConstellation) IsValid() bool {
	return o.ID() != 0
}

type EveCorporation struct {
	AllianceID    int32     `json:"alliance_id"`
	CeoID         int32     `json:"ceo_id"`
	CorporationID int32     `json:"corporation_id"`
	MemberCount   int32     `json:"member_count"`
	Name          string    `json:"name"`
	Ticker        string    `json:"ticker"`
	Timestamp     time.Time `json:"timestamp"`
}

func (o EveCorporation) ID() int32 {
	return o.CorporationID
}

func (o EveCorporation) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-day))
}

func (o EveCorporation) IsNPC() bool {
	if o.CorporationID >= npcCorporationIDBegin && o.CorporationID < npcCorporationIDEnd {
		return true
	}
	return false
}

func (o EveCorporation) IsValid() bool {
	return o.ID() != 0
}

type EveFaction struct {
	CorporationID        int32     `json:"corporation_id"`
	FactionID            int32     `json:"faction_id"`
	MilitiaCorporationID int32     `json:"militia_corporation_id"`
	Name                 string    `json:"name"`
	Timestamp            time.Time `json:"timestamp"`
}

func (o EveFaction) ID() int32 {
	return o.FactionID
}

func (o EveFaction) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-week))
}

func (o EveFaction) IsValid() bool {
	return o.ID() != 0
}

type EveGroup struct {
	CategoryID int32     `json:"category_id"`
	GroupID    int32     `json:"group_id"`
	Name       string    `json:"name"`
	Published  bool      `json:"published"`
	Timestamp  time.Time `json:"timestamp"`
}

func (o EveGroup) ID() int32 {
	return o.GroupID
}

func (o EveGroup) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-week))
}

func (o EveGroup) IsValid() bool {
	return o.ID() != 0
}

type EveRegion struct {
	Name      string    `json:"name"`
	RegionID  int32     `json:"region_id"`
	Timestamp time.Time `json:"timestamp"`
}

func (o EveRegion) ID() int32 {
	return o.RegionID
}

func (o EveRegion) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-week))
}

func (o EveRegion) IsValid() bool {
	return o.ID() != 0
}

type EveType struct {
	GroupID   int32     `json:"group_id"`
	Name      string    `json:"name"`
	Published bool      `json:"published"`
	Timestamp time.Time `json:"timestamp"`
	TypeID    int32     `json:"type_id"`
}

func (o EveType) ID() int32 {
	return o.TypeID
}

func (o EveType) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-week))
}

func (o EveType) IsValid() bool {
	return o.ID() != 0
}

type EveSolarSystem struct {
	ConstellationID int32     `json:"constellation_id"`
	Name            string    `json:"name"`
	Security        float32   `json:"security"`
	SolarSystemID   int32     `json:"system_id"`
	Timestamp       time.Time `json:"timestamp"`
}

func (o EveSolarSystem) ID() int32 {
	return o.SolarSystemID
}

func (o EveSolarSystem) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-week))
}

func (o EveSolarSystem) IsValid() bool {
	return o.ID() != 0
}

type EveStation struct {
	Name          string    `json:"name"`
	OwnerID       int32     `json:"owner_id"`
	SolarSystemID int32     `json:"system_id"`
	StationID     int32     `json:"station_id"`
	Timestamp     time.Time `json:"timestamp"`
	TypeID        int32     `json:"type_id"`
}

func (o EveStation) ID() int32 {
	return o.StationID
}

func (o EveStation) IsStale() bool {
	return o.Timestamp.Before(time.Now().UTC().Add(-week))
}

func (o EveStation) IsValid() bool {
	return o.ID() != 0
}
