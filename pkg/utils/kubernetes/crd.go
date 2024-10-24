// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"time"

	"golang.org/x/exp/maps"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	// CRDWaitTimeout specifies the total time to wait for CRDs to become ready.
	CRDWaitTimeout = 15 * time.Second
)

// CRDDeployer is a DeployWaiter that can deploy CRDs and wait for them to be ready.
type CRDDeployer struct {
	client            client.Client
	applier           kubernetes.Applier
	crdNameToManifest map[string]string
}

// NewCRDDeployer returns a DeployWaiter that can deploy CRDs and wait for them to be ready.
func NewCRDDeployer(client client.Client, applier kubernetes.Applier, manifests []string) (*CRDDeployer, error) {
	crdNameToManifest, err := MakeCRDNameMap(manifests)
	if err != nil {
		return nil, err
	}
	return &CRDDeployer{
		client:            client,
		applier:           applier,
		crdNameToManifest: crdNameToManifest,
	}, nil
}

// Deploy deploys the CRDs.
func (c *CRDDeployer) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range c.crdNameToManifest {
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(resource)), kubernetes.DefaultMergeFuncs)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Destroy destroys the CRDs.
func (c *CRDDeployer) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range c.crdNameToManifest {
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(resource))))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Wait signals whether a CRD is ready or needs more time to be deployed.
func (c *CRDDeployer) Wait(ctx context.Context) error {
	return WaitUntilCRDManifestsReady(ctx, c.client, maps.Keys(c.crdNameToManifest))
}

// WaitCleanup for destruction to finish and component to be fully removed.
func (c *CRDDeployer) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, CRDWaitTimeout)
	defer cancel()

	for resourceName, _ := range c.crdNameToManifest {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
			},
		}
		err := WaitUntilResourceDeleted(timeoutCtx, c.client, crd, 1*time.Second)
		if err != nil {
			return err
		}
	}
	return nil
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

// WaitUntilCRDManifestsReady takes CRD ObjectKeys and waits for them to get ready with a timeout of 15 seconds.
func WaitUntilCRDManifestsReady(ctx context.Context, c client.Client, crdNames []string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, CRDWaitTimeout)
	defer cancel()

	return retry.Until(timeoutCtx, 1*time.Second, func(ctx context.Context) (done bool, err error) {
		for _, crdName := range crdNames {
			crd := &apiextensionsv1.CustomResourceDefinition{}

			if err := c.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
				return retry.SevereError(err)
			}

			if err := health.CheckCustomResourceDefinition(crd); err != nil {
				return retry.MinorError(err)
			}
		}
		return retry.Ok()
	})
}

// GetObjectNameFromManifest takes a manifest and returns its corresponding name.
func GetObjectNameFromManifest(manifest string) (string, error) {
	object, err := kubernetes.NewManifestReader([]byte(manifest)).Read()
	if err != nil {
		return "", err
	}
	return object.GetName(), nil
}
