package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	bolt "go.etcd.io/bbolt"
)

const (
	bucketEveCategories = "eve_categories"
	bucketEveCharacters = "eve_characters"
	bucketEveEntities   = "eve_entities"
	bucketEveGroups     = "eve_groups"
	bucketEveTypes      = "eve_types"
)

var buckets = []string{
	bucketEveCategories,
	bucketEveCharacters,
	bucketEveEntities,
	bucketEveTypes,
	bucketEveGroups,
}

type Storage struct {
	db *bolt.DB
}

func NewStorage(db *bolt.DB) *Storage {
	st := &Storage{db: db}
	return st
}

func (st *Storage) Init() error {
	if err := st.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketEveCategories)); err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketEveEntities)); err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketEveGroups)); err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketEveTypes)); err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Clear deletes all objects in all buckets.
func (st *Storage) Clear() (int, error) {
	var n int
	if err := st.db.Update(func(tx *bolt.Tx) error {
		for _, name := range buckets {
			b := tx.Bucket([]byte(name))
			if b == nil {
				return fmt.Errorf("bucket does not exist: %s", name)
			}
			c := b.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				if err := c.Delete(); err != nil {
					return err
				}
				n++
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return n, nil
}

// MustClear is like [Clear], but panics on any error.
func (st *Storage) MustClear() int {
	n, err := st.Clear()
	if err != nil {
		panic(err)
	}
	return n
}

func (st *Storage) ListFreshEveEntitiesByID(ids []int32) ([]EveEntity, []int32, error) {
	return listFreshEveObjectsByID[EveEntity](st, bucketEveEntities, ids)
}

func (st *Storage) ListFreshEveEntitiesByName(names []string) ([]EveEntity, error) {
	isMatch := make(map[string]bool)
	for _, n := range names {
		isMatch[n] = true
	}
	objs := make([]EveEntity, 0)
	if err := st.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEveEntities))
		if b == nil {
			return fmt.Errorf("bucket does not exist: %s", bucketEveEntities)
		}
		if err := b.ForEach(func(_, v []byte) error {
			var o EveEntity
			if err := json.Unmarshal(v, &o); err != nil {
				return err
			}
			if isMatch[o.Name] && !o.IsStale() {
				objs = append(objs, o)
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return objs, nil
}

func (st *Storage) ListEveCharacters() ([]EveCharacter, error) {
	return listEveObjects[EveCharacter](st, bucketEveCharacters)
}

func (st *Storage) ListFreshEveCharactersByID(ids []int32) ([]EveCharacter, []int32, error) {
	return listFreshEveObjectsByID[EveCharacter](st, bucketEveCharacters, ids)
}

func (st *Storage) UpdateOrCreateEveCharacters(objs []EveCharacter) error {
	return updateOrCreateEveObjects(st, bucketEveCharacters, objs)
}

func (st *Storage) ListEveEntities() ([]EveEntity, error) {
	return listEveObjects[EveEntity](st, bucketEveEntities)
}

func (st *Storage) UpdateOrCreateEveEntities(objs []EveEntity) error {
	return updateOrCreateEveObjects(st, bucketEveEntities, objs)
}

func (st *Storage) ListEveCategories() ([]EveCategory, error) {
	return listEveObjects[EveCategory](st, bucketEveCategories)
}

func (st *Storage) ListFreshEveCategoriesByID(ids ...int32) ([]EveCategory, []int32, error) {
	return listFreshEveObjectsByID[EveCategory](st, bucketEveCategories, ids)
}

func (st *Storage) UpdateOrCreateEveCategories(objs []EveCategory) error {
	return updateOrCreateEveObjects(st, bucketEveCategories, objs)
}

func (st *Storage) ListEveGroups() ([]EveGroup, error) {
	return listEveObjects[EveGroup](st, bucketEveGroups)
}

func (st *Storage) ListFreshEveGroupsByID(ids ...int32) ([]EveGroup, []int32, error) {
	return listFreshEveObjectsByID[EveGroup](st, bucketEveGroups, ids)
}

func (st *Storage) UpdateOrCreateEveGroups(objs []EveGroup) error {
	return updateOrCreateEveObjects(st, bucketEveGroups, objs)
}

func (st *Storage) ListEveTypes() ([]EveType, error) {
	return listEveObjects[EveType](st, bucketEveTypes)
}

func (st *Storage) ListFreshEveTypesByID(ids []int32) ([]EveType, []int32, error) {
	return listFreshEveObjectsByID[EveType](st, bucketEveTypes, ids)
}

func (st *Storage) UpdateOrCreateEveTypes(objs []EveType) error {
	return updateOrCreateEveObjects(st, bucketEveTypes, objs)
}

type Identifiable interface {
	ID() int32
}

func listEveObjects[T Identifiable](st *Storage, bucket string) ([]T, error) {
	objs := make([]T, 0)
	if err := st.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket does not exist: %s", bucket)
		}
		if err := b.ForEach(func(k, v []byte) error {
			var o T
			if err := json.Unmarshal(v, &o); err != nil {
				return err
			}
			objs = append(objs, o)
			return nil
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return objs, nil
}

type IdentifiableAndStaleness interface {
	Identifiable
	IsStale() bool
}

func listFreshEveObjectsByID[T IdentifiableAndStaleness](st *Storage, bucket string, ids []int32) ([]T, []int32, error) {
	notFound := make([]int32, 0)
	objs := make([]T, 0)
	if err := st.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket does not exist: %s", bucket)
		}
		for _, id := range ids {
			k := []byte(strconv.Itoa(int(id)))
			v := b.Get(k)
			if v == nil {
				notFound = append(notFound, id)
				continue
			}
			var o T
			if err := json.Unmarshal(v, &o); err != nil {
				return err
			}
			if o.IsStale() {
				notFound = append(notFound, id)
				continue
			}
			objs = append(objs, o)
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return objs, notFound, nil
}

func updateOrCreateEveObjects[T Identifiable](st *Storage, bucket string, objs []T) error {
	if len(objs) == 0 {
		return nil
	}
	if err := st.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket does not exist: %s", bucket)
		}
		for _, o := range objs {
			id := int(o.ID())
			if id == 0 {
				return fmt.Errorf("invalid")
			}
			v, err := json.Marshal(o)
			if err != nil {
				return err
			}
			k := strconv.Itoa(id)
			if err := b.Put([]byte(k), v); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
