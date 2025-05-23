// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// SecretObjectMetaForConfig returns the object meta structure that can be used inside the
// secret that shall contain the generated OSC output.
func SecretObjectMetaForConfig(config *extensionsv1alpha1.OperatingSystemConfig) metav1.ObjectMeta {
	var (
		name      = fmt.Sprintf("osc-result-%s", config.Name)
		namespace = config.Namespace
	)

	if cloudConfig := config.Status.CloudConfig; cloudConfig != nil {
		name = cloudConfig.SecretRef.Name
		namespace = cloudConfig.SecretRef.Namespace
	}

	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
}
