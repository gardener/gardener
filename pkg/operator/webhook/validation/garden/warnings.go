// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/api/core/shoot"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var gardenCoreScheme *runtime.Scheme

func init() {
	gardenCoreScheme = runtime.NewScheme()
	utilruntime.Must(gardencoreinstall.AddToScheme(gardenCoreScheme))
	utilruntime.Must(admissioncontrollerconfigv1alpha1.AddToScheme(gardenCoreScheme))
}

// GetWarnings returns warnings for the given Garden.
func GetWarnings(garden *operatorv1alpha1.Garden) ([]string, error) {
	var warnings []string

	if kubeAPIServer := garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; kubeAPIServer != nil {
		coreKubeAPIServerConfig := &gardencore.KubeAPIServerConfig{}
		if err := gardenCoreScheme.Convert(kubeAPIServer.KubeAPIServerConfig, coreKubeAPIServerConfig, nil); err != nil {
			return nil, err
		}

		path := field.NewPath("spec", "virtualCluster", "kubernetes", "kubeAPIServer")
		warnings = append(warnings, shoot.GetKubeAPIServerWarnings(coreKubeAPIServerConfig, path)...)
	}

	return warnings, nil
}
