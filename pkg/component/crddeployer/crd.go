// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeployer

import (
	"context"
	"time"

	"golang.org/x/exp/maps"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	// CRDWaitTimeout specifies the total time to wait for CRDs to become ready.
	CRDWaitTimeout = 15 * time.Second
)

// crdDeployer is a DeployWaiter that can deploy CRDs and wait for them to be ready.
type crdDeployer struct {
	client            client.Client
	applier           kubernetes.Applier
	crdNameToManifest map[string]string
}

// NewCRDDeployer returns a DeployWaiter that can deploy CRDs and wait for them to be ready.
func NewCRDDeployer(client client.Client, applier kubernetes.Applier, manifests []string) (component.DeployWaiter, error) {
	crdNameToManifest, err := MakeCRDNameMap(manifests)
	if err != nil {
		return nil, err
	}
	return &crdDeployer{
		client:            client,
		applier:           applier,
		crdNameToManifest: crdNameToManifest,
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

	for resourceName, _ := range c.crdNameToManifest {
		fns = append(fns, func(ctx context.Context) error {
			crd := &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
			}
			return client.IgnoreNotFound(c.client.Delete(ctx, crd))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Wait waits for the CRDs to be deployed.
func (c *crdDeployer) Wait(ctx context.Context) error {
	return WaitUntilCRDManifestsReady(ctx, c.client, maps.Keys(c.crdNameToManifest))
}

// WaitCleanup waits for destruction to finish and CRDs to be fully removed.
func (c *crdDeployer) WaitCleanup(ctx context.Context) error {
	return WaitUntilCRDManifestsDestroyed(ctx, c.client, maps.Keys(c.crdNameToManifest))
}

// MakeCRDNameMap returns a map that has the name of the resource as key, and the corresponding manifest as value.
func MakeCRDNameMap(manifests []string) (map[string]string, error) {
	crdNameToManifest := make(map[string]string)
	for _, manifest := range manifests {
		name, err := GetObjectNameFromManifest(manifest)
		if err != nil {
			return nil, err
		}
		crdNameToManifest[name] = manifest
	}
	return crdNameToManifest, nil
}

// WaitUntilCRDManifestsReady takes names of CRDs and waits for them to get ready with a timeout of 15 seconds.
func WaitUntilCRDManifestsReady(ctx context.Context, c client.Client, crdNames []string) error {
	var fns []flow.TaskFn
	for _, crdName := range crdNames {
		fns = append(fns, func(ctx context.Context) error {
			timeoutCtx, cancel := context.WithTimeout(ctx, CRDWaitTimeout)
			defer cancel()

			return retry.Until(timeoutCtx, 1*time.Second, func(ctx context.Context) (done bool, err error) {
				crd := &apiextensionsv1.CustomResourceDefinition{}

				if err := c.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
					return retry.SevereError(err)
				}

				if err := health.CheckCustomResourceDefinition(crd); err != nil {
					return retry.MinorError(err)
				}
				return retry.Ok()
			})
		})
	}
	return flow.Parallel(fns...)(ctx)
}

// WaitUntilCRDManifestsDestroyed takes names of CRDs and waits for them to get destroted with a timeout of 15 seconds.
func WaitUntilCRDManifestsDestroyed(ctx context.Context, c client.Client, crdNames []string) error {
	var fns []flow.TaskFn

	for _, resourceName := range crdNames {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
			},
		}

		fns = append(fns, func(ctx context.Context) error {
			timeoutCtx, cancel := context.WithTimeout(ctx, CRDWaitTimeout)
			defer cancel()
			return kubernetesutils.WaitUntilResourceDeleted(timeoutCtx, c, crd, 1*time.Second)
		})
	}
	return flow.Parallel(fns...)(ctx)
}

// GetObjectNameFromManifest takes a manifest and returns its corresponding name.
func GetObjectNameFromManifest(manifest string) (string, error) {
	object, err := kubernetes.NewManifestReader([]byte(manifest)).Read()
	if err != nil {
		return "", err
	}
	return object.GetName(), nil
}
