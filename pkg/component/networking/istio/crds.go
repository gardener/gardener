// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
)

var (
	//go:embed charts/istio/istio-crds/crd-all.gen.yaml
	crds string
)

// NewCRD can be used to deploy istio CRDs.
func NewCRD(
	client client.Client,
) (component.DeployWaiter, error) {
	return crddeployer.New(client, []string{crds}, false)
}
