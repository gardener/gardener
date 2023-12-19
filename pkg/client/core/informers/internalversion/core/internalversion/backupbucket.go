// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

// Code generated by informer-gen. DO NOT EDIT.

package internalversion

import (
	"context"
	time "time"

	core "github.com/gardener/gardener/pkg/apis/core"
	clientsetinternalversion "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	internalinterfaces "github.com/gardener/gardener/pkg/client/core/informers/internalversion/internalinterfaces"
	internalversion "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// BackupBucketInformer provides access to a shared informer and lister for
// BackupBuckets.
type BackupBucketInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() internalversion.BackupBucketLister
}

type backupBucketInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewBackupBucketInformer constructs a new informer for BackupBucket type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewBackupBucketInformer(client clientsetinternalversion.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredBackupBucketInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredBackupBucketInformer constructs a new informer for BackupBucket type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredBackupBucketInformer(client clientsetinternalversion.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.Core().BackupBuckets().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.Core().BackupBuckets().Watch(context.TODO(), options)
			},
		},
		&core.BackupBucket{},
		resyncPeriod,
		indexers,
	)
}

func (f *backupBucketInformer) defaultInformer(client clientsetinternalversion.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredBackupBucketInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *backupBucketInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&core.BackupBucket{}, f.defaultInformer)
}

func (f *backupBucketInformer) Lister() internalversion.BackupBucketLister {
	return internalversion.NewBackupBucketLister(f.Informer().GetIndexer())
}
