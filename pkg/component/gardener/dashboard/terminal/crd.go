// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	_ "embed"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	//go:embed assets/crd-dashboard.gardener.cloud_terminals.yaml
	rawCRD string
)

func (t *terminal) crd() (*apiextensionsv1.CustomResourceDefinition, error) {
	return kubernetesutils.DecodeCRD(rawCRD)
}
