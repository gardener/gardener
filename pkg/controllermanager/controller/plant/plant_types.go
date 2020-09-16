// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plant

import (
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"

	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
)

type defaultPlantControl struct {
	clientMap     clientmap.ClientMap
	secretsLister kubecorev1listers.SecretLister
	recorder      record.EventRecorder
	config        *config.ControllerManagerConfiguration
}

// StatusCloudInfo contains the cloud info for the plant status
type StatusCloudInfo struct {
	CloudType  string
	Region     string
	K8sVersion string
}
