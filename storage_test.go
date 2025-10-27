package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

func TestStorageEveEntites(t *testing.T) {
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
	t.Run("can list entities by ID", func(t *testing.T) {
		st.MustClear()
		ee1 := EveEntity{EntityID: 1, Name: "abc", Category: Character}
		ee2 := EveEntity{EntityID: 2, Name: "def", Category: Station}
		ee3 := EveEntity{EntityID: 3, Name: "ghi", Category: Faction}
		err = st.UpdateOrCreateEveEntities([]EveEntity{ee1, ee2, ee3})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		ee, missing, err := st.ListEveEntitiesByID([]int32{1, 3, 4})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range ee {
			got = append(got, x.EntityID)
		}
		want := []int32{1, 3}
		assert.ElementsMatch(t, want, got)
		assert.ElementsMatch(t, []int32{4}, missing)
	})
	t.Run("can list entities by Name", func(t *testing.T) {
		st.MustClear()
		ee1 := EveEntity{EntityID: 1, Name: "alpha", Category: Character}
		ee2 := EveEntity{EntityID: 2, Name: "def", Category: Station}
		ee3 := EveEntity{EntityID: 3, Name: "alpha", Category: Faction}
		err = st.UpdateOrCreateEveEntities([]EveEntity{ee1, ee2, ee3})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		ee, err := st.ListEveEntitiesByName([]string{"alpha"})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range ee {
			got = append(got, x.EntityID)
		}
		want := []int32{1, 3}
		assert.ElementsMatch(t, want, got)
	})
	t.Run("can remove stale objects", func(t *testing.T) {
		st.MustClear()
		ee1 := EveEntity{EntityID: 1, Name: "abc", Category: Character, Timestamp: time.Now()}
		ee2 := EveEntity{EntityID: 2, Name: "def", Category: Station}
		err = st.UpdateOrCreateEveEntities([]EveEntity{ee1, ee2})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		err := st.RemoveStaleObjects()
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		ee, err := st.ListEveEntities()
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range ee {
			got = append(got, x.EntityID)
		}
		want := []int32{1}
		assert.ElementsMatch(t, want, got)
	})
}

func TestStorageEveTypes(t *testing.T) {
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
	t.Run("can list objs", func(t *testing.T) {
		st.MustClear()
		oo1 := EveType{TypeID: 1, Name: "abc"}
		oo2 := EveType{TypeID: 2, Name: "def"}
		err = st.UpdateOrCreateEveTypes([]EveType{oo1, oo2})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		oo, err := st.ListEveTypes()
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range oo {
			got = append(got, x.TypeID)
		}
		want := []int32{1, 2}
		assert.ElementsMatch(t, want, got)
	})
	t.Run("can list objs by ID", func(t *testing.T) {
		st.MustClear()
		oo1 := EveType{TypeID: 1, Name: "abc"}
		oo2 := EveType{TypeID: 2, Name: "def"}
		oo3 := EveType{TypeID: 3, Name: "ghi"}
		err = st.UpdateOrCreateEveTypes([]EveType{oo1, oo2, oo3})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		ee, missing, err := st.ListEveTypesByID([]int32{1, 3, 4})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range ee {
			got = append(got, x.TypeID)
		}
		want := []int32{1, 3}
		assert.ElementsMatch(t, want, got)
		assert.ElementsMatch(t, []int32{4}, missing)

	})
}

func TestStorageEveObjects(t *testing.T) {
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
	t.Run("can list categories", func(t *testing.T) {
		st.MustClear()
		o1 := EveCategory{CategoryID: 1, Name: "abc"}
		o2 := EveCategory{CategoryID: 2, Name: "def"}
		err = st.UpdateOrCreateEveCategories([]EveCategory{o1, o2})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		oo, err := st.ListEveCategories()
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range oo {
			got = append(got, x.ID())
		}
		want := []int32{1, 2}
		assert.ElementsMatch(t, want, got)
	})
	t.Run("can list groups", func(t *testing.T) {
		st.MustClear()
		o1 := EveGroup{GroupID: 1, Name: "abc"}
		o2 := EveGroup{GroupID: 2, Name: "def"}
		err = st.UpdateOrCreateEveGroups([]EveGroup{o1, o2})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		oo, err := st.ListEveGroups()
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range oo {
			got = append(got, x.ID())
		}
		want := []int32{1, 2}
		assert.ElementsMatch(t, want, got)
	})
}
