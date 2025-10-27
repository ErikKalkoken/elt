package main

import (
	"strings"
	"time"

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
	Category  EveEntityCategory `json:"category"`
	EntityID  int32             `json:"entity_id"`
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
}

func (ee EveEntity) ID() int32 {
	return ee.EntityID
}

type EveCategory struct {
	CategoryID int32     `json:"category_id"`
	Name       string    `json:"name"`
	Published  bool      `json:"published"`
	Timestamp  time.Time `json:"timestamp"`
}

func (ec EveCategory) ID() int32 {
	return ec.CategoryID
}

type EveGroup struct {
	CategoryID int32     `json:"category_id"`
	GroupID    int32     `json:"group_id"`
	Name       string    `json:"name"`
	Published  bool      `json:"published"`
	Timestamp  time.Time `json:"timestamp"`
}

func (eg EveGroup) ID() int32 {
	return eg.GroupID
}

type EveType struct {
	GroupID   int32     `json:"group_id"`
	Name      string    `json:"name"`
	Published bool      `json:"published"`
	Timestamp time.Time `json:"timestamp"`
	TypeID    int32     `json:"type_id"`
}

func (et EveType) ID() int32 {
	return et.TypeID
}
