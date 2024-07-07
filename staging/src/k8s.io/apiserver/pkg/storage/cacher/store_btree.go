/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cacher

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/btree"
	"k8s.io/client-go/tools/cache"
)

func newThreadedBtreeStoreIndexer(indexers cache.Indexers, degree int) *threadedStoreIndexer {
	return &threadedStoreIndexer{
		store:   newBtreeStore(degree),
		indexer: newIndexer(indexers),
	}
}

type threadedStoreIndexer struct {
	lock    sync.RWMutex
	store   btreeStore
	indexer indexer
}

func (si *threadedStoreIndexer) Count(prefix, continueKey string) (count int) {
	si.lock.RLock()
	defer si.lock.RUnlock()
	return si.store.Count(prefix, continueKey)
}

func (si *threadedStoreIndexer) Clone() orderedStore {
	si.lock.RLock()
	defer si.lock.RUnlock()
	return si.store.Clone()
}

func (si *threadedStoreIndexer) Add(obj interface{}) error {
	return si.addOrUpdate(obj)
}

func (si *threadedStoreIndexer) Update(obj interface{}) error {
	return si.addOrUpdate(obj)
}

func (si *threadedStoreIndexer) addOrUpdate(obj interface{}) error {
	if obj == nil {
		return fmt.Errorf("obj cannot be nil")
	}
	newElem, ok := obj.(*storeElement)
	if !ok {
		return fmt.Errorf("obj not a storeElement: %#v", obj)
	}
	si.lock.Lock()
	defer si.lock.Unlock()
	oldElem := si.store.addOrUpdateElem(newElem)
	return si.indexer.updateElem(newElem.Key, oldElem, newElem)
}

func (si *threadedStoreIndexer) Delete(obj interface{}) error {
	storeElem, ok := obj.(*storeElement)
	if !ok {
		return fmt.Errorf("obj not a storeElement: %#v", obj)
	}
	si.lock.Lock()
	defer si.lock.Unlock()
	oldObj := si.store.deleteElem(storeElem)
	if oldObj == nil {
		return nil
	}
	return si.indexer.updateElem(storeElem.Key, oldObj.(*storeElement), nil)
}

func (si *threadedStoreIndexer) List() []interface{} {
	si.lock.RLock()
	defer si.lock.RUnlock()
	return si.store.List()
}

func (si *threadedStoreIndexer) ListPrefix(prefix, continueKey string, limit int) ([]interface{}, bool) {
	si.lock.RLock()
	defer si.lock.RUnlock()
	return si.store.ListPrefix(prefix, continueKey, limit)
}

func (si *threadedStoreIndexer) ListKeys() []string {
	si.lock.RLock()
	defer si.lock.RUnlock()
	return si.store.ListKeys()
}

func (si *threadedStoreIndexer) Get(obj interface{}) (item interface{}, exists bool, err error) {
	si.lock.RLock()
	defer si.lock.RUnlock()
	return si.store.Get(obj)
}

func (si *threadedStoreIndexer) GetByKey(key string) (item interface{}, exists bool, err error) {
	si.lock.RLock()
	defer si.lock.RUnlock()
	return si.store.GetByKey(key)
}

func (si *threadedStoreIndexer) Replace(objs []interface{}, resourceVersion string) error {
	si.lock.Lock()
	defer si.lock.Unlock()
	err := si.store.Replace(objs, resourceVersion)
	if err != nil {
		return err
	}
	return si.indexer.Replace(objs, resourceVersion)
}

func (si *threadedStoreIndexer) ByIndex(indexName, indexValue string) ([]interface{}, error) {
	si.lock.RLock()
	defer si.lock.RUnlock()
	return si.indexer.ByIndex(indexName, indexValue)
}

func newBtreeStore(degree int) btreeStore {
	return btreeStore{
		tree: btree.New(degree),
	}
}

type btreeStore struct {
	tree *btree.BTree
}

func (s *btreeStore) Add(obj interface{}) error {
	if obj == nil {
		return fmt.Errorf("obj cannot be nil")
	}
	storeElem, ok := obj.(*storeElement)
	if !ok {
		return fmt.Errorf("obj not a storeElement: %#v", obj)
	}
	s.addOrUpdateElem(storeElem)
	return nil
}

func (s *btreeStore) Update(obj interface{}) error {
	if obj == nil {
		return fmt.Errorf("obj cannot be nil")
	}
	storeElem, ok := obj.(*storeElement)
	if !ok {
		return fmt.Errorf("obj not a storeElement: %#v", obj)
	}
	s.addOrUpdateElem(storeElem)
	return nil
}

func (s *btreeStore) Delete(obj interface{}) error {
	if obj == nil {
		return fmt.Errorf("obj cannot be nil")
	}
	storeElem, ok := obj.(*storeElement)
	if !ok {
		return fmt.Errorf("obj not a storeElement: %#v", obj)
	}
	s.deleteElem(storeElem)
	return nil
}

func (s *btreeStore) deleteElem(storeElem *storeElement) interface{} {
	return s.tree.Delete(storeElem)
}

func (s *btreeStore) List() []interface{} {
	items := make([]interface{}, 0, s.tree.Len())
	s.tree.Ascend(func(i btree.Item) bool {
		items = append(items, i.(interface{}))
		return true
	})

	return items
}

func (s *btreeStore) ListKeys() []string {
	items := make([]string, 0, s.tree.Len())
	s.tree.Ascend(func(i btree.Item) bool {
		items = append(items, i.(*storeElement).Key)
		return true
	})

	return items
}

func (s *btreeStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	storeElem, ok := obj.(*storeElement)
	if !ok {
		return nil, false, fmt.Errorf("obj is not a storeElement")
	}
	item = s.tree.Get(storeElem)
	if item == nil {
		return nil, false, nil
	}

	return item, true, nil
}

func (s *btreeStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	return s.getByKey(key)
}

func (s *btreeStore) Replace(objs []interface{}, _ string) error {
	s.tree.Clear(false)
	for _, obj := range objs {
		storeElem, ok := obj.(*storeElement)
		if !ok {
			return fmt.Errorf("obj not a storeElement: %#v", obj)
		}
		s.addOrUpdateElem(storeElem)
	}
	return nil
}

// addOrUpdateLocked assumes a lock is held and is used for Add
// and Update operations.
func (s *btreeStore) addOrUpdateElem(storeElem *storeElement) *storeElement {
	oldObj := s.tree.ReplaceOrInsert(storeElem)
	if oldObj == nil {
		return nil
	}
	return oldObj.(*storeElement)
}

func (s *btreeStore) getByKey(key string) (item interface{}, exists bool, err error) {
	keyElement := &storeElement{Key: key}
	item = s.tree.Get(keyElement)

	return item, item != nil, nil
}

func (s *btreeStore) ListPrefix(prefix, continueKey string, limit int) ([]interface{}, bool) {
	if limit < 0 {
		return nil, false
	}
	if continueKey == "" {
		continueKey = prefix
	}
	var result []interface{}
	var hasMore bool

	if limit == 0 {
		s.tree.AscendGreaterOrEqual(&storeElement{Key: continueKey}, func(i btree.Item) bool {
			elementKey := i.(*storeElement).Key
			if !strings.HasPrefix(elementKey, prefix) {
				return false
			}
			result = append(result, i.(interface{}))
			return true
		})
		return result, hasMore
	}

	s.tree.AscendGreaterOrEqual(&storeElement{Key: continueKey}, func(i btree.Item) bool {
		elementKey := i.(*storeElement).Key
		if !strings.HasPrefix(elementKey, prefix) {
			return false
		}
		// TODO: Might be worth to lookup one more item to provide more accurate HasMore.
		if len(result) < limit {
			result = append(result, i.(interface{}))
		} else {
			hasMore = true
			return false
		}
		return true
	})

	return result, hasMore
}

func (s *btreeStore) Count(prefix, continueKey string) (count int) {
	if continueKey == "" {
		continueKey = prefix
	}
	s.tree.AscendGreaterOrEqual(&storeElement{Key: continueKey}, func(i btree.Item) bool {
		elementKey := i.(*storeElement).Key
		if !strings.HasPrefix(elementKey, prefix) {
			return false
		}
		count++
		return true
	})

	return count
}

func (s *btreeStore) Clone() orderedStore {
	return &btreeStore{
		tree: s.tree.Clone(),
	}
}

func newIndexer(indexers cache.Indexers) indexer {
	return indexer{
		indices:  map[string]map[string]map[string]*storeElement{},
		indexers: indexers,
	}
}

type indexer struct {
	indices  map[string]map[string]map[string]*storeElement
	indexers cache.Indexers
}

func (i *indexer) ByIndex(indexName, indexValue string) ([]interface{}, error) {
	indexFunc := i.indexers[indexName]
	if indexFunc == nil {
		return nil, fmt.Errorf("index with name %s does not exist", indexName)
	}

	index := i.indices[indexName]

	set := index[indexValue]
	list := make([]interface{}, 0, len(set))
	for _, obj := range set {
		list = append(list, obj)
	}

	return list, nil
}

func (i *indexer) Replace(objs []interface{}, resourceVersion string) error {
	i.indices = map[string]map[string]map[string]*storeElement{}
	for _, obj := range objs {
		storeElem, ok := obj.(*storeElement)
		if !ok {
			return fmt.Errorf("obj not a storeElement: %#v", obj)
		}
		err := i.updateElem(storeElem.Key, nil, storeElem)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *indexer) updateElem(key string, oldObj, newObj *storeElement) (err error) {
	var oldIndexValues, indexValues []string
	for name, indexFunc := range i.indexers {
		if oldObj != nil {
			oldIndexValues, err = indexFunc(oldObj)
		} else {
			oldIndexValues = oldIndexValues[:0]
		}
		if err != nil {
			return fmt.Errorf("unable to calculate an index entry for key %q on index %q: %w", key, name, err)
		}

		if newObj != nil {
			indexValues, err = indexFunc(newObj)
		} else {
			indexValues = indexValues[:0]
		}
		if err != nil {
			return fmt.Errorf("unable to calculate an index entry for key %q on index %q: %w", key, name, err)
		}

		index := i.indices[name]
		if index == nil {
			index = map[string]map[string]*storeElement{}
			i.indices[name] = index
		}

		if len(indexValues) == 1 && len(oldIndexValues) == 1 && indexValues[0] == oldIndexValues[0] {
			// We optimize for the most common case where indexFunc returns a single value which has not been changed
			i.add(key, indexValues[0], newObj, index)
			continue
		}

		for _, value := range oldIndexValues {
			i.delete(key, value, index)
		}
		for _, value := range indexValues {
			i.add(key, value, newObj, index)
		}
	}

	return nil
}

func (i *indexer) add(key, value string, obj *storeElement, index map[string]map[string]*storeElement) {
	set := index[value]
	if set == nil {
		set = map[string]*storeElement{}
		index[value] = set
	}
	set[key] = obj
}

func (i *indexer) delete(key, value string, index map[string]map[string]*storeElement) {
	set := index[value]
	if set == nil {
		return
	}
	delete(set, key)
	// If we don's delete the set when zero, indices with high cardinality
	// short lived resources can cause memory to increase over time from
	// unused empty sets. See `kubernetes/kubernetes/issues/84959`.
	if len(set) == 0 {
		delete(index, value)
	}
}

// continueCache caches roots of trees that were created as
// clones to serve LIST requests. When a continue request is
// meant to be served for a certain LIST request, we retrieve
// the tree that served the LIST request and serve the continue
// request from there.
//
// A tree is removed from this cache when the RV at which it was
// created is removed from the watchCache.
type continueCache struct {
	sync.RWMutex
	revisions *btree.BTree
	cache     map[uint64]orderedStore
}

func newContinueCache() *continueCache {
	return &continueCache{
		revisions: btree.New(32),
		cache:     make(map[uint64]orderedStore)}
}

func (c *continueCache) FindEqualOrLower(rv uint64) (indexer orderedStore, foundRV uint64, ok bool) {
	c.RLock()
	defer c.RUnlock()

	indexer, ok = c.cache[rv]
	if ok {
		return indexer, rv, ok
	}

	c.revisions.DescendLessOrEqual(rev(rv), func(i btree.Item) bool {
		foundRV = uint64(i.(rev))
		ok = true
		return false
	})
	if ok {
		indexer, ok = c.cache[foundRV]
	}
	return indexer, foundRV, ok
}

func (c *continueCache) Set(rv uint64, indexer orderedStore) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.cache[rv]; !ok {
		c.revisions.ReplaceOrInsert(rev(rv))
		c.cache[rv] = indexer.Clone()
	}
}

func (c *continueCache) Cleanup(rv uint64) {
	c.Lock()
	defer c.Unlock()
	for c.revisions.Len() > 0 {
		minRV := uint64(c.revisions.Min().(rev))
		if rv < minRV {
			break
		}
		if _, ok := c.cache[minRV]; ok {
			delete(c.cache, minRV)
		}
		c.revisions.DeleteMin()
	}
}

type rev uint64

func (r1 rev) Less(r2 btree.Item) bool {
	return r1 < r2.(rev)
}
