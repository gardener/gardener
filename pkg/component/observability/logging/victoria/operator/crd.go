// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
)

var (
	//go:embed templates/crd-operator.victoriametrics.com_vlsingles.yaml
	crdVLSingles string
)

// NewCRDs can be used to deploy victoria-operator CRDs.
func NewCRDs(client client.Client) (component.DeployWaiter, error) {
	resources := []string{
		crdVLSingles,
	}
	return crddeployer.New(client, resources, false)
}
