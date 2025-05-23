// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clientmap

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	"k8s.io/utils/clock"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ ClientMap = &GenericClientMap{}

const waitForCacheSyncTimeout = 5 * time.Minute

// MaxRefreshInterval is the maximum rate at which the version and hash of a single ClientSet are checked, to
// decide whether the ClientSet should be refreshed. Also, the GenericClientMap waits at least MaxRefreshInterval
// after creating a new ClientSet before checking if it should be refreshed.
var MaxRefreshInterval = 5 * time.Second

// GenericClientMap is a generic implementation of clientmap.ClientMap, which can be used by specific ClientMap
// implementations to reuse the core logic for storing, requesting, invalidating and starting ClientSets. Specific
// implementations only need to provide a ClientSetFactory that can produce new ClientSets for the respective keys
// if a corresponding entry is not found in the GenericClientMap.
type GenericClientMap struct {
	clientSets map[ClientSetKey]*clientMapEntry
	factory    ClientSetFactory

	// lock guards concurrent access to clientSets
	lock sync.RWMutex

	log logr.Logger

	clock clock.Clock

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

	// refreshLimiter limits the attempts to refresh the entry due to an outdated ClientSetHash or server version.
	refreshLimiter *rate.Limiter
}

// NewGenericClientMap creates a new GenericClientMap with the given factory and logger.
func NewGenericClientMap(factory ClientSetFactory, logger logr.Logger, clock clock.Clock) *GenericClientMap {
	return &GenericClientMap{
		clientSets: make(map[ClientSetKey]*clientMapEntry),
		factory:    factory,
		log:        logger,
		clock:      clock,
	}
}

// GetClient requests a ClientSet for a cluster identified by the given key. If the ClientSet was already created before,
// it returns the one saved in the map, otherwise it creates a new ClientSet by using the provided ClientSetFactory.
// New ClientSets are immediately started if the ClientMap has already been started before. Also GetClient will regularly
// rediscover the server version of the targeted cluster and check if the config hash has changed and recreate the
// ClientSet if a config hash change is detected.
func (cm *GenericClientMap) GetClient(ctx context.Context, key ClientSetKey) (kubernetes.Interface, error) {
	entry, found := func() (*clientMapEntry, bool) {
		cm.lock.RLock()
		defer cm.lock.RUnlock()

		entry, found := cm.clientSets[key]
		return entry, found
	}()

	if found {
		if entry.refreshLimiter.AllowN(cm.clock.Now(), 1) {
			shouldRefresh, err := func() (bool, error) {
				// invalidate client if the config of the client has changed (e.g. kubeconfig secret)
				hash, err := cm.factory.CalculateClientSetHash(ctx, key)
				if err != nil {
					return false, fmt.Errorf("failed to calculate new hash for ClientSet: %w", err)
				}

				if hash != entry.hash {
					cm.log.Info("Refreshing ClientSet due to changed ClientSetHash", "key", key.Key(), "oldHash", entry.hash, "newHash", hash)
					return true, nil
				}

				// refresh server version
				oldVersion := entry.clientSet.Version()
				serverVersion, err := entry.clientSet.DiscoverVersion()
				if err != nil {
					return false, fmt.Errorf("failed to refresh ClientSet's server version: %w", err)
				}
				if serverVersion.GitVersion != oldVersion {
					cm.log.Info("New server version discovered for ClientSet", "key", key.Key(), "serverVersion", serverVersion.GitVersion)
					// client is intentionally not refreshed in this case, see https://github.com/gardener/gardener/pull/2581 for details
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
		if entry, err = cm.addClientSet(ctx, key); err != nil {
			return nil, err
		}
	}

	return entry.clientSet, nil
}

func (cm *GenericClientMap) addClientSet(ctx context.Context, key ClientSetKey) (*clientMapEntry, error) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	// ClientSet might have been created in the meanwhile (e.g. two goroutines might concurrently call
	// GetClient() when the ClientSet is not yet created)
	if entry, found := cm.clientSets[key]; found {
		return entry, nil
	}

	cs, hash, err := cm.factory.NewClientSet(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("error creating new ClientSet for key %q: %w", key.Key(), err)
	}

	cm.log.Info("Created new ClientSet", "key", key.Key(), "hash", hash)

	entry := &clientMapEntry{
		clientSet:      cs,
		refreshLimiter: rate.NewLimiter(rate.Every(MaxRefreshInterval), 1),
		hash:           hash,
	}

	// avoid checking if the client should be refreshed directly after creating, by directly taking a token here
	entry.refreshLimiter.AllowN(cm.clock.Now(), 1)

	// add ClientSet to map
	cm.clientSets[key] = entry

	// if ClientMap is not started, then don't automatically start new ClientSets
	if cm.started {
		if err := cm.startClientSet(key, entry); err != nil {
			return nil, err
		}
	}

	return entry, nil
}

// InvalidateClient removes the ClientSet identified by the given key from the ClientMap after stopping its cache.
func (cm *GenericClientMap) InvalidateClient(key ClientSetKey) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	entry, found := cm.clientSets[key]
	if !found {
		return nil
	}

	cm.log.Info("Invalidating ClientSet", "key", key.Key())
	if entry.cancel != nil {
		entry.cancel()
	}

	delete(cm.clientSets, key)

	if invalidate, ok := cm.factory.(Invalidate); ok {
		if err := invalidate.InvalidateClient(key); err != nil {
			return err
		}
	}

	return nil
}

// Start starts the caches of all contained ClientSets and saves the stopCh to start the caches of ClientSets,
// that will be created afterwards.
func (cm *GenericClientMap) Start(ctx context.Context) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	if cm.started {
		return nil
	}
	cm.stopCh = ctx.Done()

	// start any ClientSets that have been added before starting the ClientMap
	// there will probably be only a garden client in here on startup, no other clients
	for key, entry := range cm.clientSets {
		// each call to startClientSet will also wait for the respective caches to sync.
		// doing this in the loop here is not problematic, as
		if err := cm.startClientSet(key, entry); err != nil {
			return err
		}
	}

	// set started to true, so we immediately start all newly created clientsets
	cm.started = true
	return nil
}

func (cm *GenericClientMap) startClientSet(key ClientSetKey, entry *clientMapEntry) error {
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

	// limit the amount of time to wait for a cache sync, as this can block controller worker routines
	// and we don't want to block all workers if it takes a long time to sync some caches
	waitContext, cancel := context.WithTimeout(clientSetContext, waitForCacheSyncTimeout)
	defer cancel()

	// make sure the ClientSet has synced before returning
	// callers of Start/GetClient expect that cached clients can be used immediately after retrieval from a started
	// ClientMap or after starting the ClientMap respectively
	if !waitForClientSetCacheSync(waitContext, entry) {
		return fmt.Errorf("timed out waiting for caches of ClientSet with key %q to sync", key.Key())
	}

	return nil
}

func waitForClientSetCacheSync(ctx context.Context, entry *clientMapEntry) bool {
	// don't need a lock here, as caller already holds lock
	if entry.synced {
		return true
	}

	if !entry.clientSet.WaitForCacheSync(ctx) {
		return false
	}

	entry.synced = true
	return true
}
