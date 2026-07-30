package main

import (
	"fmt"
	"slices"
	"strings"
)

type enumValue struct {
	allowed []string
	value   string
}

func newEnumValue(allowed []string, defaultVal string) *enumValue {
	return &enumValue{
		allowed: allowed,
		value:   defaultVal,
	}
}

// Set parses and validates input against the allowlist
func (e *enumValue) Set(val string) error {
	if !slices.Contains(e.allowed, val) {
		return fmt.Errorf("must be one of [%s]", strings.Join(e.allowed, ", "))
	}
	e.value = val
	return nil
}

// String returns the current value for help text displays
func (e *enumValue) String() string {
	return e.value
}

// Type specifies the value type in help output
func (e *enumValue) Type() string {
	return "string"
}

// FormatDescription generates formatted multi-line help text
func (e *enumValue) FormatDescription(summary string) string {
	return fmt.Sprintf(
		"%s (allowed: %s)",
		summary,
		strings.Join(e.allowed, ", "),
	)
}
