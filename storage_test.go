package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

func TestStorage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "everef.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := NewStorage(db)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	t.Run("can get an entity", func(t *testing.T) {
		st.MustClear()
		ee1 := EveEntity{ID: 1, Name: "abc", Category: Character}
		err = st.UpdateOrCreateEveEntities(ee1)
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		ee2, err := st.GetEveEntity(1)
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		assert.Equal(t, ee1.ID, ee2.ID)
		assert.Equal(t, ee1.Name, ee2.Name)
		assert.Equal(t, ee1.Category, ee2.Category)
	})
	t.Run("can list entities by ID", func(t *testing.T) {
		st.MustClear()
		ee1 := EveEntity{ID: 1, Name: "abc", Category: Character}
		ee2 := EveEntity{ID: 2, Name: "def", Category: Station}
		ee3 := EveEntity{ID: 3, Name: "ghi", Category: Faction}
		err = st.UpdateOrCreateEveEntities(ee1, ee2, ee3)
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		ee, err := st.ListEveEntitiesByID(1, 2)
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range ee {
			got = append(got, x.ID)
		}
		want := []int32{1, 2}
		assert.ElementsMatch(t, want, got)
	})
	t.Run("can list entities by Name", func(t *testing.T) {
		st.MustClear()
		ee1 := EveEntity{ID: 1, Name: "alpha", Category: Character}
		ee2 := EveEntity{ID: 2, Name: "def", Category: Station}
		ee3 := EveEntity{ID: 3, Name: "alpha", Category: Faction}
		err = st.UpdateOrCreateEveEntities(ee1, ee2, ee3)
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		ee, err := st.ListEveEntitiesByName("alpha")
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range ee {
			got = append(got, x.ID)
		}
		want := []int32{1, 3}
		assert.ElementsMatch(t, want, got)
	})
}
