package main

import (
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// timeouts: expired objects are considered stale
const (
	defaultTimeout = 24 * time.Hour
	eveTypeTimeout = 24 * 7 * time.Hour
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
	case InventoryType:
		return o.Timestamp.Before(time.Now().UTC().Add(-eveTypeTimeout))
	default:
		return o.Timestamp.Before(time.Now().UTC().Add(-defaultTimeout))
	}
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
	return o.Timestamp.Before(time.Now().UTC().Add(-eveTypeTimeout))
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
	return o.Timestamp.Before(time.Now().UTC().Add(-eveTypeTimeout))
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
	return o.Timestamp.Before(time.Now().UTC().Add(-eveTypeTimeout))
}
