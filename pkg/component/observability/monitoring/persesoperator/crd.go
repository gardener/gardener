// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package persesoperator

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
)

var (
	//go:embed templates/crd-perses.dev_perses.yaml
	crdPerses string
	//go:embed templates/crd-perses.dev_persesdashboards.yaml
	crdPersesDashboards string
	//go:embed templates/crd-perses.dev_persesdatasources.yaml
	crdPersesDatasources string
)

// NewCRDs can be used to deploy perses-operator CRDs.
func NewCRDs(client client.Client, applier kubernetes.Applier) (component.DeployWaiter, error) {
	resources := []string{
		crdPerses,
		crdPersesDashboards,
		crdPersesDatasources,
	}
	return crddeployer.New(client, applier, resources, false)
}
