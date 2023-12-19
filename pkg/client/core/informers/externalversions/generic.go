// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

// Code generated by informer-gen. DO NOT EDIT.

package externalversions

import (
	"fmt"

	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	cache "k8s.io/client-go/tools/cache"
)

// GenericInformer is type of SharedIndexInformer which will locate and delegate to other
// sharedInformers based on type
type GenericInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() cache.GenericLister
}

type genericInformer struct {
	informer cache.SharedIndexInformer
	resource schema.GroupResource
}

// Informer returns the SharedIndexInformer.
func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}

// Lister returns the GenericLister.
func (f *genericInformer) Lister() cache.GenericLister {
	return cache.NewGenericLister(f.Informer().GetIndexer(), f.resource)
}

// ForResource gives generic access to a shared informer of the matching type
// TODO extend this to unknown resources with a client pool
func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {
	switch resource {
	// Group=core.gardener.cloud, Version=v1beta1
	case v1beta1.SchemeGroupVersion.WithResource("backupbuckets"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().BackupBuckets().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("backupentries"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().BackupEntries().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("cloudprofiles"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().CloudProfiles().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("controllerdeployments"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().ControllerDeployments().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("controllerinstallations"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().ControllerInstallations().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("controllerregistrations"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().ControllerRegistrations().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("exposureclasses"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().ExposureClasses().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("internalsecrets"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().InternalSecrets().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("projects"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().Projects().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("quotas"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().Quotas().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("secretbindings"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().SecretBindings().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("seeds"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().Seeds().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("shoots"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().Shoots().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("shootstates"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Core().V1beta1().ShootStates().Informer()}, nil

	}

	return nil, fmt.Errorf("no informer found for %v", resource)
}
