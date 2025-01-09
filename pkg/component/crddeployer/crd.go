// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeployer

import (
	"context"

	"golang.org/x/exp/maps"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// crdDeployer is a DeployWaiter that can deploy CRDs and wait for them to be ready.
type crdDeployer struct {
	client            client.Client
	applier           kubernetes.Applier
	crdNameToManifest map[string]string
	confirmDeletion   bool
}

// New returns a new instance of DeployWaiter for CRDs.
func New(client client.Client, applier kubernetes.Applier, manifests []string, confirmDeletion bool) (component.DeployWaiter, error) {
	crdNameToManifest, err := generateNameToCRDMap(manifests)
	if err != nil {
		return nil, err
	}
	return &crdDeployer{
		client:            client,
		applier:           applier,
		crdNameToManifest: crdNameToManifest,
		confirmDeletion:   confirmDeletion,
	}, nil
}

// Deploy deploys the CRDs.
func (c *crdDeployer) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range c.crdNameToManifest {
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(resource)), kubernetes.DefaultMergeFuncs)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Destroy destroys the CRDs.
func (c *crdDeployer) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	for resourceName := range c.crdNameToManifest {
		fns = append(fns, func(ctx context.Context) error {
			crd := &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
			}

			if c.confirmDeletion {
				if err := gardenerutils.ConfirmDeletion(ctx, c.client, crd); client.IgnoreNotFound(err) != nil {
					return err
				}
			}

			return client.IgnoreNotFound(c.client.Delete(ctx, crd))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Wait waits for the CRDs to be deployed.
func (c *crdDeployer) Wait(ctx context.Context) error {
	return kubernetesutils.WaitUntilCRDManifestsReady(ctx, c.client, maps.Keys(c.crdNameToManifest)...)
}

// WaitCleanup waits for destruction to finish and CRDs to be fully removed.
func (c *crdDeployer) WaitCleanup(ctx context.Context) error {
	return kubernetesutils.WaitUntilCRDManifestsDestroyed(ctx, c.client, maps.Keys(c.crdNameToManifest)...)
}

// generateNameToCRDMap returns a map that has the name of the resource as key, and the corresponding manifest as value.
func generateNameToCRDMap(manifests []string) (map[string]string, error) {
	crdNameToManifest := make(map[string]string)
	for _, manifest := range manifests {
		object, err := kubernetes.NewManifestReader([]byte(manifest)).Read()
		if err != nil {
			return nil, err
		}
		crdNameToManifest[object.GetName()] = manifest
	}
	return crdNameToManifest, nil
}
