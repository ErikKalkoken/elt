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
	bucketEveEntities  = "eve_entities"
	eveEntitiesTimeout = 24 * time.Hour // older objects will no longer be found
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
		b, err := tx.CreateBucketIfNotExists([]byte(bucketEveEntities))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		// remove stale objects from database
		var n int
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			ee, err := eveEntityFromDB(v)
			if err != nil {
				return err
			}
			if ee.Timestamp.Before(time.Now().UTC().Add(-eveEntitiesTimeout)) {
				if err := c.Delete(); err != nil {
					return err
				}
				n++
			}
		}
		slog.Debug("stale objects deleted", "count", n)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (st *Storage) Clear() (int, error) {
	var n int
	if err := st.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEveEntities))
		if b == nil {
			return fmt.Errorf("bucket does not exit: %s", bucketEveEntities)
		}
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if err := c.Delete(); err != nil {
				return err
			}
			n++
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return n, nil
}

func (st *Storage) MustClear() int {
	n, err := st.Clear()
	if err != nil {
		panic(err)
	}
	return n
}

func (st *Storage) GetEveEntity(id int32) (EveEntity, error) {
	var ee EveEntity
	if err := st.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEveEntities))
		if b == nil {
			return fmt.Errorf("bucket does not exit: %s", bucketEveEntities)
		}
		v := b.Get(eveEntityKeyToDB(id))
		if v == nil {
			return ErrNotFound
		}
		ee2, err := eveEntityFromDB(v)
		if err != nil {
			return err
		}
		ee = ee2
		return nil
	}); err != nil {
		return ee, err
	}
	return ee, nil
}

func (st *Storage) ListEveEntities() ([]EveEntity, error) {
	entities := make([]EveEntity, 0)
	if err := st.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEveEntities))
		if b == nil {
			return fmt.Errorf("bucket does not exit: %s", bucketEveEntities)
		}
		if err := b.ForEach(func(k, v []byte) error {
			ee, err := eveEntityFromDB(v)
			if err != nil {
				return err
			}
			entities = append(entities, ee)
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

func (st *Storage) ListEveEntitiesByID(ids ...int32) ([]EveEntity, error) {
	isMatch := make(map[int32]bool)
	for _, id := range ids {
		isMatch[id] = true
	}
	entities := make([]EveEntity, 0)
	if err := st.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEveEntities))
		if b == nil {
			return fmt.Errorf("bucket does not exit: %s", bucketEveEntities)
		}
		if err := b.ForEach(func(k, v []byte) error {
			id, err := eveEntityKeyFromDB(k)
			if err != nil {
				return err
			}
			if isMatch[id] {
				ee, err := eveEntityFromDB(v)
				if err != nil {
					return err
				}
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

func (st *Storage) UpdateOrCreateEveEntities(entities ...EveEntity) error {
	if len(entities) == 0 {
		return nil
	}
	if err := st.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEveEntities))
		if b == nil {
			return fmt.Errorf("bucket does not exit: %s", bucketEveEntities)
		}
		for _, ee := range entities {
			ee.Timestamp = time.Now().UTC()
			v, err := eveEntityToDB(ee)
			if err != nil {
				return err
			}
			if err := b.Put(eveEntityKeyToDB(ee.ID), v); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func eveEntityKeyToDB(id int32) []byte {
	k := strconv.Itoa(int(id))
	return []byte(k)
}

func eveEntityKeyFromDB(b []byte) (int32, error) {
	id, err := strconv.Atoi(string(b))
	if err != nil {
		return 0, err
	}
	return int32(id), nil
}

func eveEntityToDB(ee EveEntity) ([]byte, error) {
	return json.Marshal(ee)
}

func eveEntityFromDB(b []byte) (EveEntity, error) {
	var ee EveEntity
	if err := json.Unmarshal(b, &ee); err != nil {
		return ee, err
	}
	return ee, nil
}
