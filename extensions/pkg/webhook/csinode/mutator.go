//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package csinode

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
)

const (
	errMsg = "failed to mutate CSINode %s: %v"
)

// NewMutator creates a new accelerated network Mutator.
func NewMutator(mgr manager.Manager, logger logr.Logger, args *Args) webhook.MutatorWithShootClient {
	return &Mutator{
		client:   mgr.GetClient(),
		logger:   logger,
		drivers:  args.Drivers,
		provider: args.Provider,
	}
}

// CSINodeMutateFunc mutates the given CSINodeDriver in-place
type CSINodeMutateFunc func(logr.Logger, *storagev1.CSINodeDriver, *extensionsv1alpha1.WorkerPool, *extensions.Cluster) error

// Mutator implements the required interface for a mutating webhook using the shoot client.
type Mutator struct {
	client   client.Client
	logger   logr.Logger
	drivers  map[string]CSINodeMutateFunc
	provider string
}

// Mutate validates and if needed mutates the given object.
func (m *Mutator) Mutate(ctx context.Context, new, old client.Object, sc client.Client, cluster *extensions.Cluster) error {
	var (
		cnNew, cnOld *storagev1.CSINode
		ok           bool
	)

	if m.provider != cluster.Shoot.GetProviderType() {
		return nil
	}

	cnNew, ok = new.(*storagev1.CSINode)
	if !ok {
		return fmt.Errorf("could not mutate: new object is not of kind %q", "CSINode")
	}
	if old != nil {
		cnOld, ok = old.(*storagev1.CSINode)
		if !ok {
			return fmt.Errorf("could not mutate: new object is not of kind %q", "CSINode")
		}
	}

Outer:
	for idx, driver := range cnNew.Spec.Drivers {
		// skip drivers that we do not care to mutate.
		if _, ok := m.drivers[driver.Name]; !ok {
			continue
		}

		if cnOld != nil {
			for _, oldDriver := range cnOld.Spec.Drivers {
				// skip CSI Drivers that we have already adapted
				if oldDriver.Name == driver.Name {
					m.logger.Info("Tried to update CSINode, but the driver is already there... skipping")
					continue Outer
				}
			}
		}

		nodeName := new.GetName()
		node := &corev1.Node{}
		err := sc.Get(ctx, client.ObjectKey{Name: nodeName}, node)
		if err != nil {
			return err
		}

		var poolName string
		if poolName, ok = node.GetLabels()[constants.LabelWorkerPool]; !ok {
			return fmt.Errorf(errMsg, nodeName, "can't find worker pool label on the node object")
		}

		worker := &extensionsv1alpha1.Worker{}
		err = m.client.Get(ctx, client.ObjectKey{
			Namespace: cluster.ObjectMeta.Name,
			Name:      cluster.Shoot.GetName(),
		}, worker)
		if err != nil {
			return err
		}

		for _, pool := range worker.Spec.Pools {
			if pool.Name != poolName {
				continue
			}

			err := m.drivers[driver.Name](m.logger, &cnNew.Spec.Drivers[idx], &pool, cluster)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GenericCSINodeMutate is a generic way to implement the csi node allocatable calculation by subtracting data volumes.
func GenericCSINodeMutate(log logr.Logger, driver *storagev1.CSINodeDriver, pool *extensionsv1alpha1.WorkerPool, _ *extensions.Cluster) error {
	if pool.DataVolumes != nil {
		if driver.Allocatable != nil && driver.Allocatable.Count != nil {
			newCount := *driver.Allocatable.Count - int32(len(pool.DataVolumes))
			// this is a special case due to how k8s treats the 0 value. Currently, 0 represents that infinite volumes can be attached to a node.
			// see also https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1859 for a similar treatment.
			// This creates a potential issue for nodes whose dataVolumes is equal to the attach limit.
			if newCount <= 0 {
				newCount = 1
			}
			driver.Allocatable.Count = &newCount
			log.Info("Set allocatable field for csi driver", "driver", driver.Name, "value", *driver.Allocatable.Count)
			return nil
		}
	}

	return nil
}
