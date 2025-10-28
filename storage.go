package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	bolt "go.etcd.io/bbolt"
)

//go:generate go run ./tools/genstorage EveType EveGroup EveCategory EveEntity EveCharacter

const (
	bucketEveCategory  = "eve_categories"
	bucketEveCharacter = "eve_characters"
	bucketEveEntity    = "eve_entities"
	bucketEveGroup     = "eve_groups"
	bucketEveType      = "eve_types"
)

var bucketNames = []string{
	bucketEveCategory,
	bucketEveCharacter,
	bucketEveEntity,
	bucketEveType,
	bucketEveGroup,
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
		for _, n := range bucketNames {
			if _, err := tx.CreateBucketIfNotExists([]byte(n)); err != nil {
				return fmt.Errorf("create bucket %s: %s", n, err)
			}
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
		for _, name := range bucketNames {
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

func (st *Storage) ListFreshEveEntitiesByName(names []string) ([]EveEntity, error) {
	isMatch := make(map[string]bool)
	for _, n := range names {
		isMatch[n] = true
	}
	objs := make([]EveEntity, 0)
	if err := st.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEveEntity))
		if b == nil {
			return fmt.Errorf("bucket does not exist: %s", bucketEveEntity)
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

type EveObject interface {
	ID() int32
	IsStale() bool
}

func listEveObjects[T EveObject](st *Storage, bucket string) ([]T, error) {
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

func listFreshEveObjectsByID[T EveObject](st *Storage, bucket string, ids []int32) ([]T, []int32, error) {
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

func updateOrCreateEveObjects[T EveObject](st *Storage, bucket string, objs []T) error {
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
