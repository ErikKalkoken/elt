package main

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ErikKalkoken/go-set"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func TestStorage_EveEntites(t *testing.T) {
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
			o.EntityID = int64(lastEntityID.Add(1))
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
		require.NoError(t, err)
		var got set.Set[int64]
		for _, x := range ee {
			got.Add(x.EntityID)
		}
		want := set.Of(o1.ID(), o2.ID(), o3.ID())
		assert.True(t, got.Equal(want), "got %q, wanted %q", got, want)
	})
	t.Run("can list fresh entities by ID", func(t *testing.T) {
		st.MustClear()
		createEveEntity(EveEntity{EntityID: 1})
		createEveEntity(EveEntity{EntityID: 2})
		createEveEntity(EveEntity{EntityID: 3})
		createEveEntity(EveEntity{EntityID: 4, Timestamp: time.Now().Add(-1000 * time.Hour)})
		ee, missing, err := st.ListFreshEveEntityByID(set.Of[int64](1, 3, 4, 5))
		require.NoError(t, err)
		var got set.Set[int64]
		for _, x := range ee {
			got.Add(x.ID())
		}
		want := set.Of[int64](1, 3)
		assert.True(t, got.Equal(want), "got %q, wanted %q", got, want)
		wantMissing := set.Of[int64](4, 5)
		assert.True(t, missing.Equal(wantMissing), "got %q, wanted %q", wantMissing, want)
	})
	t.Run("can list fresh entities by Name", func(t *testing.T) {
		st.MustClear()
		o1 := createEveEntity(EveEntity{Name: "alpha"})
		o2 := createEveEntity(EveEntity{Name: "alpha"})
		createEveEntity(EveEntity{Name: "bravo"})
		createEveEntity(EveEntity{Name: "alpha", Timestamp: time.Now().Add(-1000 * time.Hour)})
		ee, err := st.ListFreshEveEntitiesByName([]string{"alpha"})
		require.NoError(t, err)
		var got set.Set[int64]
		for _, x := range ee {
			got.Add(x.EntityID)
		}
		want := set.Of(o1.ID(), o2.ID())
		assert.True(t, got.Equal(want), "got %q, wanted %q", got, want)
	})
	t.Run("should return error when trying to create object with ID 0", func(t *testing.T) {
		st.MustClear()
		o := EveEntity{EntityID: 0, Name: "abc", Category: CategoryCharacter}
		err := st.UpdateOrCreateEveEntity([]EveEntity{o})
		assert.Error(t, err)
	})
}

// TestStorage_EveTypes represents the tests for all generated methods.
func TestStorage_EveTypes(t *testing.T) {
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
			o.TypeID = int64(lastTypeID.Add(1))
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
		require.NoError(t, err)
		oo, err := st.ListEveType()
		require.NoError(t, err)
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
		require.NoError(t, err)
		oo, err := st.ListEveType()
		require.NoError(t, err)
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
		ee, missing, err := st.ListFreshEveTypeByID(set.Of[int64](1, 3, 4))
		require.NoError(t, err)
		var got set.Set[int64]
		for _, x := range ee {
			got.Add(x.TypeID)
		}
		want := set.Of[int64](1, 3)
		assert.True(t, got.Equal(want), "got %q, wanted %q", got, want)
		wantMissing := set.Of[int64](4)
		assert.True(t, missing.Equal(wantMissing), "got %q, wanted %q", wantMissing, want)
	})
}

func TestStorage_EveToken(t *testing.T) {
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
	t.Run("should create a new token", func(t *testing.T) {
		st.MustClear()
		tok1 := EveToken{
			AccessToken:   "AccessToken",
			CharacterID:   42,
			CharacterName: "CharacterName",
			ExpiresAt:     time.Now().UTC(),
			RefreshToken:  "RefreshToken",
			Scopes:        []string{"abc", "def"},
			TokenType:     "TokenType",
		}
		err = st.UpdateOrCreateEveToken(tok1)
		require.NoError(t, err)
		tok2, err := st.GetEveToken()
		require.NoError(t, err)
		assert.Equal(t, tok1, tok2)
	})
	t.Run("should update existing token", func(t *testing.T) {
		st.MustClear()
		tok1 := EveToken{
			AccessToken:   "AccessToken",
			CharacterID:   42,
			CharacterName: "CharacterName",
			ExpiresAt:     time.Now().UTC(),
			RefreshToken:  "RefreshToken",
			Scopes:        []string{"abc", "def"},
			TokenType:     "TokenType",
		}
		err = st.UpdateOrCreateEveToken(tok1)
		require.NoError(t, err)
		tok2 := EveToken{
			AccessToken:   "AccessToken1",
			CharacterID:   43,
			CharacterName: "CharacterName1",
			ExpiresAt:     time.Now().UTC(),
			RefreshToken:  "RefreshToken1",
			Scopes:        []string{"abc1", "def1"},
			TokenType:     "TokenType1",
		}
		err = st.UpdateOrCreateEveToken(tok2)
		tok3, err := st.GetEveToken()
		require.NoError(t, err)
		assert.Equal(t, tok2, tok3)
	})
	t.Run("should return error when token not found", func(t *testing.T) {
		st.MustClear()
		_, err := st.GetEveToken()
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
