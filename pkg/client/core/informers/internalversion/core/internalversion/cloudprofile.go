// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

// CloudProfileInformer provides access to a shared informer and lister for
// CloudProfiles.
type CloudProfileInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() internalversion.CloudProfileLister
}

type cloudProfileInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewCloudProfileInformer constructs a new informer for CloudProfile type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCloudProfileInformer(client clientsetinternalversion.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCloudProfileInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredCloudProfileInformer constructs a new informer for CloudProfile type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCloudProfileInformer(client clientsetinternalversion.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.Core().CloudProfiles().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.Core().CloudProfiles().Watch(context.TODO(), options)
			},
		},
		&core.CloudProfile{},
		resyncPeriod,
		indexers,
	)
}

func (f *cloudProfileInformer) defaultInformer(client clientsetinternalversion.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCloudProfileInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *cloudProfileInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&core.CloudProfile{}, f.defaultInformer)
}

func (f *cloudProfileInformer) Lister() internalversion.CloudProfileLister {
	return internalversion.NewCloudProfileLister(f.Informer().GetIndexer())
}
