// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EnsureSecretChecksumAnnotation ensures that the given pod template has an annotation containing the checksum of the
// secret with the given name and namespace.
func EnsureSecretChecksumAnnotation(ctx context.Context, template *corev1.PodTemplateSpec, c client.Client, namespace, name string) error {
	// Get secret from cluster
	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret); err != nil {
		return fmt.Errorf("could not get secret '%s/%s': %w", namespace, name, err)
	}

	// Add checksum annotation
	metav1.SetMetaDataAnnotation(&template.ObjectMeta, "checksum/secret-"+name, utils.ComputeChecksum(secret.Data))
	return nil
}

// EnsureConfigMapChecksumAnnotation ensures that the given pod template has an annotation containing the checksum of the
// configmap with the given name and namespace.
func EnsureConfigMapChecksumAnnotation(ctx context.Context, template *corev1.PodTemplateSpec, c client.Client, namespace, name string) error {
	// Get configmap from cluster
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm); err != nil {
		return fmt.Errorf("could not get configmap '%s/%s': %w", namespace, name, err)
	}

	// Add checksum annotation
	metav1.SetMetaDataAnnotation(&template.ObjectMeta, "checksum/configmap-"+name, utils.ComputeChecksum(cm.Data))
	return nil
}
