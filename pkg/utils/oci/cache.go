// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import "sync"

var defaultCache = newCache()

type cacher interface {
	Get(key string) ([]byte, bool)
	Set(key string, blob []byte)
}

func newCache() *cache {
	return &cache{
		items: map[string][]byte{},
	}
}

// cache is a basic key-value cache i.e. a map protected by a mutex. Items are never removed from the cache.
type cache struct {
	mu    sync.RWMutex
	items map[string][]byte
}

func (c *cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	blob, found := c.items[key]
	c.mu.RUnlock()
	return blob, found
}

func (c *cache) Set(key string, blob []byte) {
	c.mu.Lock()
	c.items[key] = blob
	c.mu.Unlock()
}
