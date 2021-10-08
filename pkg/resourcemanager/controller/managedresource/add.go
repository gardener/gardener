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

package managedresource

import (
	"context"
	"fmt"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionshandler "github.com/gardener/gardener/extensions/pkg/handler"
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	gardenerconstantsv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcemanagercmd "github.com/gardener/gardener/pkg/resourcemanager/cmd"
	"github.com/gardener/gardener/pkg/resourcemanager/mapper"
	managerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of the managedresource controller.
const ControllerName = "resource-controller"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerOptions are options for adding the controller to a Manager.
type ControllerOptions struct {
	maxConcurrentWorkers int
	syncPeriod           time.Duration
	resourceClass        string
	alwaysUpdate         bool
	clusterID            string
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	MaxConcurrentWorkers      int
	SyncPeriod                time.Duration
	ClassFilter               *managerpredicate.ClassFilter
	AlwaysUpdate              bool
	ClusterID                 string
	GarbageCollectorActivated bool

	TargetClientConfig resourcemanagercmd.TargetClientConfig
}

// AddToManagerWithOptions adds the controller to a Manager with the given config.
func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	mgr.GetLogger().Info("Used cluster id: " + conf.ClusterID)
	c, err := controller.New(ControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
		Reconciler: extensionscontroller.OperationAnnotationWrapper(
			func() client.Object { return &resourcesv1alpha1.ManagedResource{} },
			&Reconciler{
				targetClient:              conf.TargetClientConfig.Client,
				targetRESTMapper:          conf.TargetClientConfig.RESTMapper,
				targetScheme:              conf.TargetClientConfig.Scheme,
				class:                     conf.ClassFilter,
				alwaysUpdate:              conf.AlwaysUpdate,
				syncPeriod:                conf.SyncPeriod,
				clusterID:                 conf.ClusterID,
				garbageCollectorActivated: conf.GarbageCollectorActivated,
			},
		),
	})
	if err != nil {
		return fmt.Errorf("unable to set up managedresource controller: %w", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &resourcesv1alpha1.ManagedResource{}},
		&handler.EnqueueRequestForObject{},
		conf.ClassFilter, predicate.Or(
			predicate.GenerationChangedPredicate{},
			extensionspredicate.HasOperationAnnotation(),
			managerpredicate.ConditionStatusChanged(resourcesv1alpha1.ResourcesHealthy, managerpredicate.ConditionChangedToUnhealthy),
		),
	); err != nil {
		return fmt.Errorf("unable to watch ManagedResources: %w", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		extensionshandler.EnqueueRequestsFromMapper(mapper.SecretToManagedResourceMapper(conf.ClassFilter), extensionshandler.UpdateWithOldAndNew),
	); err != nil {
		return fmt.Errorf("unable to watch Secrets mapping to ManagedResources: %w", err)
	}
	return nil
}

// AddToManager adds the controller to a Manager using the default config.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, defaultControllerConfig)
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&o.maxConcurrentWorkers, "max-concurrent-workers", 10, "number of worker threads for concurrent reconciliation of resources")
	fs.DurationVar(&o.syncPeriod, "sync-period", time.Minute, "duration how often existing resources should be synced")
	fs.StringVar(&o.resourceClass, "resource-class", managerpredicate.DefaultClass, "resource class used to filter resource resources")
	fs.StringVar(&o.clusterID, "cluster-id", "", "optional cluster id for source cluster")
	fs.BoolVar(&o.alwaysUpdate, "always-update", false, "if set to false then a resource will only be updated if its desired state differs from the actual state. otherwise, an update request will be always sent.")
}

// Complete completes the given command line flags and set the defaultControllerConfig accordingly.
func (o *ControllerOptions) Complete() error {
	if o.resourceClass == "" {
		o.resourceClass = managerpredicate.DefaultClass
	}

	defaultControllerConfig = ControllerConfig{
		MaxConcurrentWorkers: o.maxConcurrentWorkers,
		SyncPeriod:           o.syncPeriod,
		ClassFilter:          managerpredicate.NewClassFilter(o.resourceClass),
		AlwaysUpdate:         o.alwaysUpdate,
		ClusterID:            o.clusterID,
	}
	return nil
}

// Completed returns the completed ControllerConfig.
func (o *ControllerOptions) Completed() *ControllerConfig {
	return &defaultControllerConfig
}

// ApplyClassFilter sets filter to the ClassFilter of this config.
func (c *ControllerConfig) ApplyClassFilter(filter *managerpredicate.ClassFilter) {
	*filter = *c.ClassFilter
}

// ApplyDefaultClusterId sets the cluster id according to a dedicated cluster access
func (c *ControllerConfig) ApplyDefaultClusterId(ctx context.Context, log logr.Logger, restcfg *rest.Config) error {
	if c.ClusterID == "<cluster>" || c.ClusterID == "<default>" {
		log.Info("Trying to get cluster id from cluster")
		tmpClient, err := client.New(restcfg, client.Options{})
		if err == nil {
			c.ClusterID, err = determineClusterIdentity(ctx, tmpClient, c.ClusterID == "<cluster>")
		}
		if err != nil {
			return fmt.Errorf("unable to determine cluster id: %+v", err)
		}
	}
	return nil
}

// determineClusterIdentity is used to extract the cluster identity from the cluster-identity
// config map. This is intended as fallback if no explicit cluster identity is given.
// in  seed-shoot scenario, the cluster id for the managed resources must be explicitly given
// to support the migration of a shoot from one seed to another. Here the identity `seed` should
// be set.
func determineClusterIdentity(ctx context.Context, c client.Client, force bool) (string, error) {
	cm := corev1.ConfigMap{}
	err := c.Get(ctx, client.ObjectKey{Name: gardenerconstantsv1beta1.ClusterIdentity, Namespace: metav1.NamespaceSystem}, &cm)
	if err == nil {
		if id, ok := cm.Data[gardenerconstantsv1beta1.ClusterIdentity]; ok {
			return id, nil
		}
		if force {
			return "", fmt.Errorf("cannot determine cluster identity from configmap: no cluster-identity entry ")
		}
	} else {
		if force || !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("cannot determine cluster identity from configmap: %s", err)
		}
	}
	return "", nil
}
