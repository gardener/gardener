// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
)

var _ clientmap.ClientMap = &GenericClientMap{}

const (
	waitForCacheSyncTimeout = 5 * time.Minute
)

var (
	// MaxRefreshInterval is the maximum rate at which the version and hash of a single ClientSet are checked, to
	// decide whether the ClientSet should be refreshed. Also, the GenericClientMap waits at least MaxRefreshInterval
	// after creating a new ClientSet before checking if it should be refreshed.
	MaxRefreshInterval = 5 * time.Second
)

// GenericClientMap is a generic implementation of clientmap.ClientMap, which can be used by specific ClientMap
// implementations to reuse the core logic for storing, requesting, invalidating and starting ClientSets. Specific
// implementations only need to provide a ClientSetFactory that can produce new ClientSets for the respective keys
// if a corresponding entry is not found in the GenericClientMap.
type GenericClientMap struct {
	clientSets map[clientmap.ClientSetKey]*clientMapEntry
	factory    clientmap.ClientSetFactory

	// lock guards concurrent access to clientSets
	lock sync.RWMutex

	log logrus.FieldLogger

	// stopCh is saved on the first call to Start and is used to start the caches of newly created ClientSets.
	stopCh  <-chan struct{}
	started bool
}

// clientMapEntry is a single entry of the ClientMap.
type clientMapEntry struct {
	clientSet kubernetes.Interface
	synced    bool
	cancel    context.CancelFunc
	hash      string

	// refreshLimiter limits the attempts to refresh the entry due to an outdated server version.
	refreshLimiter *rate.Limiter
}

// NewGenericClientMap creates a new GenericClientMap with the given factory and logger.
func NewGenericClientMap(factory clientmap.ClientSetFactory, logger logrus.FieldLogger) *GenericClientMap {
	return &GenericClientMap{
		clientSets: make(map[clientmap.ClientSetKey]*clientMapEntry),
		factory:    factory,
		log:        logger,
	}
}

// GetClient requests a ClientSet for a cluster identified by the given key. If the ClientSet was already created before,
// it returns the one saved in the map, otherwise it creates a new ClientSet by using the provided ClientSetFactory.
// New ClientSets are immediately started if the ClientMap has already been started before. Also GetClient will regularly
// rediscover the server version of the targeted cluster and check if the config hash has changed and recreate the
// ClientSet if a config hash change is detected.
func (cm *GenericClientMap) GetClient(ctx context.Context, key clientmap.ClientSetKey) (kubernetes.Interface, error) {
	entry, started, found := func() (*clientMapEntry, bool, bool) {
		cm.lock.RLock()
		defer cm.lock.RUnlock()

		entry, found := cm.clientSets[key]
		return entry, cm.started, found
	}()

	if found {
		if entry.refreshLimiter.Allow() {
			shouldRefresh, err := func() (bool, error) {
				// refresh server version
				oldVersion := entry.clientSet.Version()
				serverVersion, err := entry.clientSet.DiscoverVersion()
				if err != nil {
					return false, fmt.Errorf("failed to refresh ClientSet's server version: %w", err)
				}
				if serverVersion.GitVersion != oldVersion {
					cm.log.Infof("New server version discovered for ClientSet with key %q: %s", key.Key(), serverVersion.GitVersion)
				}

				// invalidate client if the config of the client has changed (e.g. kubeconfig secret)
				hash, err := cm.factory.CalculateClientSetHash(ctx, key)
				if err != nil {
					return false, fmt.Errorf("failed to calculate new hash for ClientSet: %w", err)
				}

				if hash != entry.hash {
					cm.log.Infof("Refreshing ClientSet with key %q due to changed ClientSetHash: %s/%s", key.Key(), entry.hash, hash)
					return true, nil
				}

				return false, nil
			}()
			if err != nil {
				return nil, err
			}

			if shouldRefresh {
				if err := cm.InvalidateClient(key); err != nil {
					return nil, fmt.Errorf("error refreshing ClientSet for key %q: %w", key.Key(), err)
				}
				found = false
			}
		}
	}

	if !found {
		var err error
		if entry, started, err = cm.addClientSet(ctx, key); err != nil {
			return nil, err
		}
	}

	if started {
		// limit the amount of time to wait for a cache sync, as this can block controller worker routines
		// and we don't want to block all workers if it takes a long time to sync some caches
		waitContext, cancel := context.WithTimeout(ctx, waitForCacheSyncTimeout)
		defer cancel()

		// make sure the ClientSet has synced before returning
		if !waitForClientSetCacheSync(waitContext, entry) {
			return nil, fmt.Errorf("timed out waiting for caches of ClientSet with key %q to sync", key.Key())
		}
	}

	return entry.clientSet, nil
}

func (cm *GenericClientMap) addClientSet(ctx context.Context, key clientmap.ClientSetKey) (*clientMapEntry, bool, error) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	// ClientSet might have been created in the meanwhile (e.g. two goroutines might concurrently call
	// GetClient() when the ClientSet is not yet created)
	if entry, found := cm.clientSets[key]; found {
		return entry, cm.started, nil
	}

	cm.log.Infof("Creating new ClientSet for key %q", key.Key())
	cs, err := cm.factory.NewClientSet(ctx, key)
	if err != nil {
		return nil, false, fmt.Errorf("error creating new ClientSet for key %q: %w", key.Key(), err)
	}

	// save hash of client set configuration to detect if it should be recreated later on
	hash, err := cm.factory.CalculateClientSetHash(ctx, key)
	if err != nil {
		return nil, false, fmt.Errorf("error calculating ClientSet hash for key %q: %w", key.Key(), err)
	}

	entry := &clientMapEntry{
		clientSet:      cs,
		refreshLimiter: rate.NewLimiter(rate.Every(MaxRefreshInterval), 1),
		hash:           hash,
	}

	// avoid checking if the client should be refreshed directly after creating, by directly taking a token here
	entry.refreshLimiter.Allow()

	// add ClientSet to map
	cm.clientSets[key] = entry

	// if ClientMap is not started, then don't automatically start new ClientSets
	if cm.started {
		cm.startClientSet(entry)
	}

	return entry, cm.started, nil
}

// InvalidateClient removes the ClientSet identified by the given key from the ClientMap after stopping its cache.
func (cm *GenericClientMap) InvalidateClient(key clientmap.ClientSetKey) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	entry, found := cm.clientSets[key]
	if !found {
		return nil
	}

	cm.log.Infof("Invalidating ClientSet for key %q", key.Key())
	if entry.cancel != nil {
		entry.cancel()
	}

	delete(cm.clientSets, key)

	if invalidate, ok := cm.factory.(clientmap.Invalidate); ok {
		if err := invalidate.InvalidateClient(key); err != nil {
			return err
		}
	}

	return nil
}

// Start starts the caches of all contained ClientSets and saves the stopCh to start the caches of ClientSets,
// that will be created afterwards.
func (cm *GenericClientMap) Start(stopCh <-chan struct{}) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	if cm.started {
		return nil
	}

	cm.stopCh = stopCh

	for _, entry := range cm.clientSets {
		cm.startClientSet(entry)
	}

	// set started to true, so we immediately start all newly created clientsets
	cm.started = true
	return nil
}

func (cm *GenericClientMap) startClientSet(entry *clientMapEntry) {
	clientSetContext, clientSetCancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-clientSetContext.Done():
		case <-cm.stopCh:
			clientSetCancel()
		}
	}()

	entry.cancel = clientSetCancel

	entry.clientSet.Start(clientSetContext)
}

func waitForClientSetCacheSync(ctx context.Context, entry *clientMapEntry) bool {
	// We don't need a lock here, as waiting in multiple goroutines is not harmful.
	// But RLocking here (for every GetClient) would be blocking creating new clients, so we should avoid that.
	if entry.synced {
		return true
	}

	if !entry.clientSet.WaitForCacheSync(ctx) {
		return false
	}

	entry.synced = true
	return true
}
