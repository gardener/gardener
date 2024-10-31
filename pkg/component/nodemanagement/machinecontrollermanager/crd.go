// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager

import (
	"context"
	_ "embed"
	"fmt"

	"golang.org/x/exp/maps"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubernetesutils "github.com/gardener/gardener/pkg/component/crddeployer"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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

	crdNameToManifest map[string]string
)

func init() {
	var err error
	resources := []string{
		machineClassCRD,
		machineDeploymentCRD,
		machineSetCRD,
		machineCRD,
	}
	crdNameToManifest, err = kubernetesutils.MakeCRDNameMap(resources)
	utilruntime.Must(err)
}

type crd struct {
	component.DeployWaiter
	client  client.Client
	applier kubernetes.Applier
}

// NewCRD can be used to deploy the CRD definitions for the machine-controller-manager.
func NewCRD(client client.Client, applier kubernetes.Applier) (component.DeployWaiter, error) {
	crdDeployer, err := kubernetesutils.NewCRDDeployer(client, applier, maps.Values(crdNameToManifest))
	if err != nil {
		return nil, err
	}
	return &crd{
		DeployWaiter: crdDeployer,
		client:       client,
		applier:      applier,
	}, nil
}

func (c *crd) Destroy(ctx context.Context) error {
	for _, resource := range crdNameToManifest {
		reader := kubernetes.NewManifestReader([]byte(resource))

		obj, err := reader.Read()
		if err != nil {
			return fmt.Errorf("failed reading manifest: %w", err)
		}

		if err := gardenerutils.ConfirmDeletion(ctx, c.client, obj); client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := c.client.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}
