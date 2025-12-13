// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeployer

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// crdDeployer is a DeployWaiter that can deploy CRDs and wait for them to be ready.
type crdDeployer struct {
	client             client.Client
	crdNameToCRD       map[string]*apiextensionsv1.CustomResourceDefinition
	deletionProtection bool
}

// New returns a new instance of DeployWaiter for CRDs.
func New(client client.Client, manifests []string, deletionProtection bool) (component.DeployWaiter, error) {
	// Split manifests into individual object manifests, in case multiple CRDs are provided in a single string.
	var splitManifests []string
	for _, manifest := range manifests {
		splitManifests = append(splitManifests, strings.Split(manifest, "\n---\n")...)
	}

	crdNameToCRD, err := generateCRDNameToCRDMap(splitManifests)
	if err != nil {
		return nil, err
	}

	return &crdDeployer{
		client:             client,
		crdNameToCRD:       crdNameToCRD,
		deletionProtection: deletionProtection,
	}, nil
}

// Deploy deploys the CRDs.
func (c *crdDeployer) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, desiredCRD := range c.crdNameToCRD {
		fns = append(fns, func(ctx context.Context) error {
			crd := &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: desiredCRD.Name,
				},
			}

			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, crd,
				func() error {
					crd.Labels = desiredCRD.Labels
					if c.deletionProtection {
						metav1.SetMetaDataLabel(&crd.ObjectMeta, gardenerutils.DeletionProtected, "true")
					}

					crd.Annotations = desiredCRD.Annotations
					crd.Spec = desiredCRD.Spec

					return nil
				},
				// Not sending an empty patch goes against the recommendation in the Kubernetes Clients in Gardener guide.
				// See https://github.com/gardener/gardener/blob/62ce73bd39cc2ff5ae8d711ce5d66f80cbbe2d00/docs/development/kubernetes-clients.md?plain=1#L352
				//
				// However, empty patches can be skipped here for the following reasons:
				// - The fake client (`sigs.k8s.io/controller-runtime/pkg/client/fake`), used in unit tests, eats up a lot of CPU
				//   when working with large resources during the handling of patch operations in the underlying `ObjectTracker` interface,
				//   ref: https://github.com/kubernetes/client-go/blob/master/testing/fixture.go#L48-L80
				// - Reduce the load on the kube-apiserver for CRDs that have already been deployed.
				// - CRDs are not expected to be handled by any mutating webhooks.
				controllerutils.SkipEmptyPatch{})
			return err
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Destroy destroys the CRDs.
func (c *crdDeployer) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	for resourceName := range c.crdNameToCRD {
		fns = append(fns, func(ctx context.Context) error {
			crd := &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
			}

			if c.deletionProtection {
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
	return kubernetesutils.WaitUntilCRDManifestsReady(ctx, c.client, maps.Keys(c.crdNameToCRD)...)
}

// WaitCleanup waits for destruction to finish and CRDs to be fully removed.
func (c *crdDeployer) WaitCleanup(ctx context.Context) error {
	return kubernetesutils.WaitUntilCRDManifestsDestroyed(ctx, c.client, maps.Keys(c.crdNameToCRD)...)
}

// generateCRDNameToCRDMap returns a map that has the name of the resource as key, and the corresponding CRD as value.
func generateCRDNameToCRDMap(manifests []string) (map[string]*apiextensionsv1.CustomResourceDefinition, error) {
	crdNameToCRD := make(map[string]*apiextensionsv1.CustomResourceDefinition, len(manifests))
	for _, manifest := range manifests {
		crdObj, err := kubernetesutils.DecodeCRD(manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to decode CRD: %w", err)
		}
		crdNameToCRD[crdObj.GetName()] = crdObj
	}
	return crdNameToCRD, nil
}
