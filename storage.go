package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	bucketEveEntities   = "eve_entities"
	bucketEveCategories = "eve_categories"
	bucketEveGroups     = "eve_groups"
	bucketEveTypes      = "eve_types"
	objectsTimeout      = 24 * time.Hour // expired objects should be replaced
)

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

type eveObjectWithTimestamp struct {
	Timestamp time.Time `json:"timestamp"`
}

// RemoveStaleObjects deletes all stale objects in all buckets.
func (st *Storage) RemoveStaleObjects() error {
	var n int
	if err := st.db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range []string{bucketEveEntities, bucketEveTypes, bucketEveGroups, bucketEveCategories} {
			b := tx.Bucket([]byte(bucket))
			if b == nil {
				return fmt.Errorf("bucket does not exit: %s", bucket)
			}
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				var o eveObjectWithTimestamp
				if err := json.Unmarshal(v, &o); err != nil {
					return err
				}
				if o.Timestamp.Before(time.Now().UTC().Add(-objectsTimeout)) {
					if err := c.Delete(); err != nil {
						return err
					}
					n++
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	slog.Debug("stale objects deleted", "count", n)
	return nil
}

// Clear deletes all objects in all buckets.
func (st *Storage) Clear() (int, error) {
	var n int
	if err := st.db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{bucketEveEntities, bucketEveTypes} {
			b := tx.Bucket([]byte(name))
			if b == nil {
				return fmt.Errorf("bucket does not exit: %s", name)
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

func (st *Storage) ListEveEntitiesByID(ids ...int32) ([]EveEntity, []int32, error) {
	return listEveObjectsByID[EveEntity](st, bucketEveEntities, ids)
}

func (st *Storage) ListEveEntitiesByName(names ...string) ([]EveEntity, error) {
	isMatch := make(map[string]bool)
	for _, n := range names {
		isMatch[n] = true
	}
	entities := make([]EveEntity, 0)
	if err := st.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEveEntities))
		if b == nil {
			return fmt.Errorf("bucket does not exit: %s", bucketEveEntities)
		}
		if err := b.ForEach(func(_, v []byte) error {
			ee, err := eveEntityFromDB(v)
			if err != nil {
				return err
			}
			if isMatch[ee.Name] {
				entities = append(entities, ee)
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return entities, nil
}

func (st *Storage) ListEveEntities() ([]EveEntity, error) {
	return listEveObjects[EveEntity](st, bucketEveEntities)
}

func (st *Storage) UpdateOrCreateEveEntities(objs ...EveEntity) error {
	return updateOrCreateEveObjects(st, bucketEveEntities, objs)
}

func (st *Storage) ListEveCategories() ([]EveCategory, error) {
	return listEveObjects[EveCategory](st, bucketEveCategories)
}

func (st *Storage) ListEveCategoriesByID(ids ...int32) ([]EveCategory, []int32, error) {
	return listEveObjectsByID[EveCategory](st, bucketEveCategories, ids)
}

func (st *Storage) UpdateOrCreateEveCategories(objs ...EveCategory) error {
	return updateOrCreateEveObjects(st, bucketEveCategories, objs)
}

func (st *Storage) ListEveGroups() ([]EveGroup, error) {
	return listEveObjects[EveGroup](st, bucketEveGroups)
}

func (st *Storage) ListEveGroupsByID(ids ...int32) ([]EveGroup, []int32, error) {
	return listEveObjectsByID[EveGroup](st, bucketEveGroups, ids)
}

func (st *Storage) UpdateOrCreateEveGroups(objs ...EveGroup) error {
	return updateOrCreateEveObjects(st, bucketEveGroups, objs)
}

func (st *Storage) ListEveTypes() ([]EveType, error) {
	return listEveObjects[EveType](st, bucketEveTypes)
}

func (st *Storage) ListEveTypesByID(ids ...int32) ([]EveType, []int32, error) {
	return listEveObjectsByID[EveType](st, bucketEveTypes, ids)
}

func (st *Storage) UpdateOrCreateEveTypes(objs ...EveType) error {
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
			return fmt.Errorf("bucket does not exit: %s", bucket)
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

func listEveObjectsByID[T Identifiable](st *Storage, bucket string, ids []int32) ([]T, []int32, error) {
	isMatch := make(map[int32]bool)
	for _, id := range ids {
		isMatch[id] = true
	}
	objs := make([]T, 0)
	if err := st.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket does not exit: %s", bucket)
		}
		if err := b.ForEach(func(k, v []byte) error {
			var o T
			if err := json.Unmarshal(v, &o); err != nil {
				return err
			}
			if isMatch[o.ID()] {
				objs = append(objs, o)
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	found := make(map[int32]bool)
	for _, o := range objs {
		found[o.ID()] = true
	}
	missing := make([]int32, 0)
	for _, id := range ids {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	return objs, missing, nil
}

func updateOrCreateEveObjects[T Identifiable](st *Storage, bucket string, objs []T) error {
	if len(objs) == 0 {
		return nil
	}
	if err := st.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket does not exit: %s", bucket)
		}
		for _, ee := range objs {
			v, err := json.Marshal(ee)
			if err != nil {
				return err
			}
			id := strconv.Itoa(int(ee.ID()))
			if err := b.Put([]byte(id), v); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func eveEntityFromDB(b []byte) (EveEntity, error) {
	var ee EveEntity
	if err := json.Unmarshal(b, &ee); err != nil {
		return ee, err
	}
	return ee, nil
}
