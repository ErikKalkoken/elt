package main

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

func TestStorageEveEntites(t *testing.T) {
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

	var lastEntityID atomic.Int64
	createEveEntity := func(arg ...EveEntity) EveEntity {
		var o EveEntity
		if len(arg) > 0 {
			o = arg[0]
		}
		if o.Timestamp.IsZero() {
			o.Timestamp = time.Now().UTC()
		}
		if o.EntityID == 0 {
			o.EntityID = int32(lastEntityID.Add(1))
		}
		if o.Name == "" {
			o.Name = fmt.Sprintf("Dummy #%d", o.EntityID)
		}
		if o.Category == CategoryUndefined {
			o.Category = CategoryCharacter
		}
		err := st.UpdateOrCreateEveEntity([]EveEntity{o})
		if err != nil {
			panic(err)
		}
		return o
	}
	t.Run("can list all entities", func(t *testing.T) {
		st.MustClear()
		o1 := createEveEntity()
		o2 := createEveEntity()
		o3 := createEveEntity()
		ee, err := st.ListEveEntity()
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range ee {
			got = append(got, x.EntityID)
		}
		want := []int32{o1.ID(), o2.ID(), o3.ID()}
		assert.ElementsMatch(t, want, got)
	})
	t.Run("can list fresh entities by ID", func(t *testing.T) {
		st.MustClear()
		createEveEntity(EveEntity{EntityID: 1})
		createEveEntity(EveEntity{EntityID: 2})
		createEveEntity(EveEntity{EntityID: 3})
		createEveEntity(EveEntity{EntityID: 4, Timestamp: time.Now().Add(-1000 * time.Hour)})
		ee, missing, err := st.ListFreshEveEntityByID([]int32{1, 3, 4, 5})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range ee {
			got = append(got, x.ID())
		}
		want := []int32{1, 3}
		assert.ElementsMatch(t, want, got)
		assert.ElementsMatch(t, []int32{4, 5}, missing)
	})
	t.Run("can list fresh entities by Name", func(t *testing.T) {
		st.MustClear()
		o1 := createEveEntity(EveEntity{Name: "alpha"})
		o2 := createEveEntity(EveEntity{Name: "alpha"})
		createEveEntity(EveEntity{Name: "bravo"})
		createEveEntity(EveEntity{Name: "alpha", Timestamp: time.Now().Add(-1000 * time.Hour)})
		ee, err := st.ListFreshEveEntitiesByName([]string{"alpha"})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		got := make([]int32, 0)
		for _, x := range ee {
			got = append(got, x.EntityID)
		}
		want := []int32{o1.ID(), o2.ID()}
		assert.ElementsMatch(t, want, got)
	})
	t.Run("should return error when trying to create object with ID 0", func(t *testing.T) {
		st.MustClear()
		o := EveEntity{EntityID: 0, Name: "abc", Category: CategoryCharacter}
		err := st.UpdateOrCreateEveEntity([]EveEntity{o})
		assert.Error(t, err)
	})
}

// TestStorageEveTypes represents the tests for all generated methods.
func TestStorageEveTypes(t *testing.T) {
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
	var lastTypeID atomic.Int64
	createEveType := func(arg ...EveType) EveType {
		var o EveType
		if len(arg) > 0 {
			o = arg[0]
		}
		if o.Timestamp.IsZero() {
			o.Timestamp = time.Now().UTC()
		}
		if o.TypeID == 0 {
			o.TypeID = int32(lastTypeID.Add(1))
		}
		if o.Name == "" {
			o.Name = fmt.Sprintf("Type #%d", o.TypeID)
		}
		err := st.UpdateOrCreateEveType([]EveType{o})
		if err != nil {
			panic(err)
		}
		return o
	}
	t.Run("can create new objects", func(t *testing.T) {
		st.MustClear()
		o1 := EveType{TypeID: 7, Name: "Dummy"}
		err := st.UpdateOrCreateEveType([]EveType{o1})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		oo, err := st.ListEveType()
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		assert.Len(t, oo, 1)
		o2 := oo[0]
		assert.Equal(t, o1.TypeID, o2.TypeID)
		assert.Equal(t, o1.Name, o2.Name)
	})
	t.Run("can update existing objects", func(t *testing.T) {
		st.MustClear()
		o1 := createEveType(EveType{TypeID: 7, Name: "Dummy"})
		o1.Name = "Bravo"
		err := st.UpdateOrCreateEveType([]EveType{o1})
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		oo, err := st.ListEveType()
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		assert.Len(t, oo, 1)
		o2 := oo[0]
		assert.Equal(t, o1.TypeID, o2.TypeID)
		assert.Equal(t, "Bravo", o2.Name)
	})
	t.Run("can list objs by ID", func(t *testing.T) {
		st.MustClear()
		createEveType(EveType{TypeID: 1})
		createEveType(EveType{TypeID: 2})
		createEveType(EveType{TypeID: 3})
		ee, missing, err := st.ListFreshEveTypeByID([]int32{1, 3, 4})
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
