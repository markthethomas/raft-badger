package raftbadgerdb

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/dgraph-io/badger"
	"github.com/hashicorp/raft"
)

var (
	// Bucket names we perform transactions in
	dbLogsPrefix = []byte("logs")
	dbConfPrefix = []byte("conf")

	// ErrKeyNotFound is an error indicating a given key does not exist
	ErrKeyNotFound = errors.New("not found")
)

// BadgerStore provides access to Badger for Raft to store and retrieve
// log entries. It also provides key/value storage, and can be used as
// a LogStore and StableStore. See https://godoc.org/github.com/hashicorp/raft#StableStore
// and https://godoc.org/github.com/hashicorp/raft#LogStore
type BadgerStore struct {
	db   *badger.DB
	path string
}

// Options contains all the configuraiton used to open the BoltDB
type Options struct {
	// BadgerOptions contains any Badger-specific options
	BadgerOptions badger.Options
	// Path is the directory
	Path string
}

// NewBadgerStore takes a file path and returns a connected Raft backend.
func NewBadgerStore(path string) (*BadgerStore, error) {
	return New(Options{Path: path})
}

// New uses the supplied options to open the BoltDB and prepare it for use as a raft backend.
func New(options Options) (*BadgerStore, error) {
	options.BadgerOptions = badger.DefaultOptions
	options.BadgerOptions.Dir = options.Path + "/badger"
	options.BadgerOptions.ValueDir = options.Path + "/badger"
	db, err := badger.Open(options.BadgerOptions)
	if err != nil {
		log.Fatal(err)
	}

	store := &BadgerStore{
		db:   db,
		path: options.Path,
	}
	return store, nil
}

// Close is used to gracefully close the DB connection.
func (b *BadgerStore) Close() error {
	return b.db.Close()
}

func bytesToUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

// Converts a uint to a byte slice
func uint64ToBytes(u uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, u)
	return buf
}

// FirstIndex returns the first known index from the Raft log.
func (b *BadgerStore) FirstIndex() (uint64, error) {
	first := uint64(0)
	err := b.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		it.Seek(dbLogsPrefix)
		if it.ValidForPrefix(dbLogsPrefix) {
			item := it.Item()
			k := string(item.Key()[len(dbLogsPrefix):])
			idx, err := strconv.ParseUint(k, 10, 64)
			if err != nil {
				return err
			}
			first = idx
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return first, nil
}

// LastIndex returns the last known index from the Raft log.
func (b *BadgerStore) LastIndex() (uint64, error) {
	last := uint64(0)
	if err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()
		// ensure reverse seeking will include the
		// see https://github.com/dgraph-io/badger/issues/436 and
		// https://github.com/dgraph-io/badger/issues/347
		seekKey := append(dbLogsPrefix, 0xFF)
		it.Seek(seekKey)
		if it.ValidForPrefix(dbLogsPrefix) {
			item := it.Item()
			k := string(item.Key()[len(dbLogsPrefix):])
			idx, err := strconv.ParseUint(k, 10, 64)
			if err != nil {
				return err
			}
			last = idx
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return last, nil
}

// GetLog is used to retrieve a log from Badger at a given index.
func (b *BadgerStore) GetLog(idx uint64, log *raft.Log) error {
	return b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("%s%d", dbLogsPrefix, idx)))
		if item == nil {
			return raft.ErrLogNotFound
		}
		v, err := item.Value()
		if err != nil {
			return err
		}
		buf := bytes.NewBuffer(v)
		dec := gob.NewDecoder(buf)
		return dec.Decode(&log)
	})
}

// StoreLog is used to store a single raft log
func (b *BadgerStore) StoreLog(log *raft.Log) error {
	return b.StoreLogs([]*raft.Log{log})
}

// StoreLogs is used to store a set of raft logs
func (b *BadgerStore) StoreLogs(logs []*raft.Log) error {
	return b.db.Update(func(txn *badger.Txn) error {
		for _, log := range logs {
			key := []byte(fmt.Sprintf("%s%d", dbLogsPrefix, log.Index))
			var out bytes.Buffer
			enc := gob.NewEncoder(&out)
			enc.Encode(log)
			if err := txn.Set(key, out.Bytes()); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteRange is used to delete logs within a given range inclusively.
func (b *BadgerStore) DeleteRange(min, max uint64) error {
	return b.db.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		it.Rewind()
		// Get the key to start at
		minKey := []byte(fmt.Sprintf("%s%d", dbLogsPrefix, min))
		// it.Seek(minKey)
		for it.Seek(minKey); it.ValidForPrefix(dbLogsPrefix); it.Next() {
			item := it.Item()
			// get the index as a strong to convert to uint64
			k := string(item.Key()[len(dbLogsPrefix):])
			idx, err := strconv.ParseUint(k, 10, 64)
			if err != nil {
				return err
			}
			// Handle out-of-range index
			if idx > max {
				break
			}
			// Delete in-range index
			delKey := []byte(fmt.Sprintf("%s%d", dbLogsPrefix, idx))
			if err := txn.Delete(delKey); err != nil {
				return err
			}
		}
		return nil
	})
}

// Set is used to set a key/value set outside of the raft log
func (b *BadgerStore) Set(k, v []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		key := []byte(fmt.Sprintf("%s%d", dbConfPrefix, k))
		return txn.Set(key, v)
	})
}

// Get is used to retrieve a value from the k/v store by key
func (b *BadgerStore) Get(k []byte) ([]byte, error) {
	txn := b.db.NewTransaction(true)
	defer txn.Discard()
	key := []byte(fmt.Sprintf("%s%d", dbConfPrefix, k))
	item, err := txn.Get(key)
	if item == nil {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	v, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	if err := txn.Commit(nil); err != nil {
		return nil, err
	}
	return append([]byte(nil), v...), nil
}

// SetUint64 is like Set, but handles uint64 values
func (b *BadgerStore) SetUint64(key []byte, val uint64) error {
	return b.Set(key, uint64ToBytes(val))
}

// GetUint64 is like Get, but handles uint64 values
func (b *BadgerStore) GetUint64(key []byte) (uint64, error) {
	val, err := b.Get(key)
	if err != nil {
		return 0, err
	}
	return bytesToUint64(val), nil
}
