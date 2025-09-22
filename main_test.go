package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputParser(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		out   []int32
		valid bool
	}{
		{"single number", "1", []int32{1}, true},
		{"list of numbers", "1,2", []int32{1, 2}, true},
		{"largest number", "2147483647", []int32{2_147_483_647}, true},
		{"number too large", "2147483648", nil, false},
		{"empty string", "", nil, false},
		{"text value", "x", nil, false},
		{"mixed values", "1,x", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseInput(tc.in)
			if tc.valid {
				if !assert.NoError(t, err) {
					t.Fatal()
				}
				assert.Equal(t, tc.out, got)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
