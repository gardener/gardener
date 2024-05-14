// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import "sync"

var defaultCache = newCache()

type cacher interface {
	Get(k string) ([]byte, bool)
	Set(k string, blob []byte)
}

func newCache() *cache {
	return &cache{
		items: map[string][]byte{},
	}
}

// cache is a basic key-value cache i.e. a map propected by a mutex. Items are never removed from the cache.
type cache struct {
	mu    sync.RWMutex
	items map[string][]byte
}

func (c *cache) Get(k string) ([]byte, bool) {
	c.mu.RLock()
	blob, found := c.items[k]
	c.mu.RUnlock()
	return blob, found
}

func (c *cache) Set(k string, blob []byte) {
	c.mu.Lock()
	c.items[k] = blob
	c.mu.Unlock()
}
