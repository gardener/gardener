// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certmanagement

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	//go:embed assets/crd-cert.gardener.cloud_certificaterevocations.yaml
	crdRevocations string
	//go:embed assets/crd-cert.gardener.cloud_certificates.yaml
	crdCertificates string
	//go:embed assets/crd-cert.gardener.cloud_issuers.yaml
	crdIssuers string
)

// NewCRDs can be used to deploy the CRD definitions for the cert-management.
func NewCRDs(client client.Client, applier kubernetes.Applier) (component.DeployWaiter, error) {
	return kubernetesutils.NewCRDDeployer(client, applier, []string{crdRevocations, crdCertificates, crdIssuers})
}
