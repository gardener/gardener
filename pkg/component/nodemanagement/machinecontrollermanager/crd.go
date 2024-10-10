// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	//go:embed templates/crd-machine.sapcloud.io_machineclasses.yaml
	machineClassCRD string
	//go:embed templates/crd-machine.sapcloud.io_machinedeployments.yaml
	machineDeploymentCRD string
	//go:embed templates/crd-machine.sapcloud.io_machinesets.yaml
	machineSetCRD string
	//go:embed templates/crd-machine.sapcloud.io_machines.yaml
	machineCRD string

	crdResources []string
)

func init() {
	crdResources = []string{
		machineClassCRD,
		machineDeploymentCRD,
		machineSetCRD,
		machineCRD,
	}
}

type crd struct {
	client  client.Client
	applier kubernetes.Applier
}

// NewCRD can be used to deploy the CRD definitions for the machine-controller-manager.
func NewCRD(client client.Client, applier kubernetes.Applier) component.DeployWaiter {
	return &crd{
		client:  client,
		applier: applier,
	}
}

// Deploy creates and updates the CRD definitions for the machine-controller-manager.
func (c *crd) Deploy(ctx context.Context) error {
	for _, resource := range crdResources {
		if err := c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(resource)), kubernetes.DefaultMergeFuncs); err != nil {
			return err
		}
	}

	return nil
}

func (c *crd) Destroy(ctx context.Context) error {
	for _, resource := range crdResources {
		reader := kubernetes.NewManifestReader([]byte(resource))

		obj, err := reader.Read()
		if err != nil {
			return fmt.Errorf("failed reading manifest: %w", err)
		}

		if err := gardenerutils.ConfirmDeletion(ctx, c.client, obj); client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := c.applier.DeleteManifest(ctx, reader); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

var (
	// IntervalWaitForCRD is the interval used while waiting for the CRDs to become healthy
	// or deleted.
	IntervalWaitForCRD = 1 * time.Second
	// TimeoutWaitForCRD is the timeout used while waiting for the CRDs to become healthy
	// or deleted.
	TimeoutWaitForCRD = 15 * time.Second
	// Until is an alias for retry.Until. Exposed for tests.
	Until = retry.Until
)

// Wait signals whether a CRD is ready or needs more time to be deployed.
func (c *crd) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForCRD)
	defer cancel()

	return retry.Until(timeoutCtx, IntervalWaitForCRD, func(ctx context.Context) (done bool, err error) {
		for _, resource := range crdResources {
			r := resource
			crd := &v1.CustomResourceDefinition{}

			obj, err := kubernetes.NewManifestReader([]byte(r)).Read()
			if err != nil {
				return retry.SevereError(err)
			}

			if err := c.client.Get(ctx, client.ObjectKeyFromObject(obj), crd); client.IgnoreNotFound(err) != nil {
				return retry.SevereError(err)
			}

			if err := health.CheckCustomResourceDefinition(crd); err != nil {
				return retry.MinorError(err)
			}
		}
		return retry.Ok()
	})
}

// WaitCleanup for destruction to finish and component to be fully removed. crdDeployer does not need to wait for cleanup.
func (c *crd) WaitCleanup(_ context.Context) error {
	return nil
}
