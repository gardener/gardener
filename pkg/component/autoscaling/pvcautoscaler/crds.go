// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pvcautoscaler

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
)

var (
	//go:embed assets/crd-autoscaling.gardener.cloud_persistentvolumeclaimautoscalers.yaml
	persistentVolumeClaimAutoscalerCRD string
)

// NewCRDs can be used to deploy PVC Autoscaler CRDs
func NewCRDs(client client.Client) (component.DeployWaiter, error) {
	resources := []string{
		persistentVolumeClaimAutoscalerCRD,
	}

	return crddeployer.New(client, resources, false)
}
