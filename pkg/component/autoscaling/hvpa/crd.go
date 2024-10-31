// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hvpa

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
)

var (
	//go:embed templates/crd-autoscaling.k8s.io_hvpas.yaml
	crdHvpas string
)

// NewCRD can be used to deploy the CRD definitions for the HVPA controller.
func NewCRD(client client.Client, applier kubernetes.Applier) (component.DeployWaiter, error) {
	return crddeployer.NewCRDDeployer(client, applier, []string{crdHvpas})
}
