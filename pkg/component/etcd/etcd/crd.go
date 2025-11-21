// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	druidcorecrds "github.com/gardener/etcd-druid/api/core/v1alpha1/crds"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
)

// NewCRD can be used to deploy the CRD definitions for all CRDs defined by etcd-druid.
func NewCRD(client client.Client, k8sVersion *semver.Version) (component.DeployWaiter, error) {
	crdYAMLs, err := druidcorecrds.GetAll(k8sVersion.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd-druid CRDs for Kubernetes version %s: %w", k8sVersion, err)
	}
	crdStrings := make([]string, 0, len(crdYAMLs))
	for _, crdYAML := range crdYAMLs {
		crdStrings = append(crdStrings, crdYAML)
	}
	return crddeployer.New(client, crdStrings, true)
}
