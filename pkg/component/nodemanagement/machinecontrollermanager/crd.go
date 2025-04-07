// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
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
)

// NewCRD can be used to deploy the CRD definitions for the machine-controller-manager.
func NewCRD(client client.Client, applier kubernetes.Applier) (component.DeployWaiter, error) {
	crdResources := []string{
		machineClassCRD,
		machineDeploymentCRD,
		machineSetCRD,
		machineCRD,
	}
	return crddeployer.New(client, applier, crdResources, true)
}
