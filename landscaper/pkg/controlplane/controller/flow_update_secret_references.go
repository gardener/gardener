// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateSecretReferences updates supplied secret references with used certificates.
// This is only relevant for certificate rotation, where the secret-supplied certificate had to be rotated and
// therefore needs to be updated in the source secret.
// At this point in the reconciliation flow, we know all CA and TLS certificates must be set.
func (o *operation) UpdateSecretReferences(ctx context.Context) error {
	// Gardener API Server CA
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef != nil {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef.Name,
			Namespace: o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef.Namespace,
		}}

		s.Data = map[string][]byte{
			secretDataKeyCACrt: []byte(*o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt),
		}

		if o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key != nil {
			s.Data[secretDataKeyCAKey] = []byte(*o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key)
		}

		if err := o.runtimeClient.Client().Patch(ctx, s, client.MergeFrom(s)); err != nil {
			return fmt.Errorf("failed to update secret reference in the runtime cluster (%s/%s) for the Gardener API Server CA", o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef.Namespace, o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef.Name)
		}
	}

	// Gardener Admission Controller CA
	if o.imports.GardenerAdmissionController.Enabled && o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef != nil {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef.Name,
			Namespace: o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef.Namespace,
		}}

		s.Data = map[string][]byte{
			secretDataKeyCACrt: []byte(*o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt),
		}

		if o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key != nil {
			s.Data[secretDataKeyCAKey] = []byte(*o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key)
		}

		if err := o.runtimeClient.Client().Patch(ctx, s, client.MergeFrom(s)); err != nil {
			return fmt.Errorf("failed to update secret reference in the runtime cluster (%s/%s) for the Gardener Admission Controller CA", o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef.Namespace, o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef.Name)
		}
	}

	// TLS certificates
	if o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef != nil {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef.Name,
			Namespace: o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef.Namespace,
		}}

		s.Data = map[string][]byte{
			secretDataKeyTLSCrt: []byte(*o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt),
			secretDataKeyTLSKey: []byte(*o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Key),
		}

		if err := o.runtimeClient.Client().Patch(ctx, s, client.MergeFrom(s)); err != nil {
			return fmt.Errorf("failed to update the secret reference in the runtime cluster (%s/%s) containing the TLS certificate for the Gardener API Server", o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef.Namespace, o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef.Name)
		}
	}

	if o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef != nil {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef.Name,
			Namespace: o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef.Namespace,
		}}

		s.Data = map[string][]byte{
			secretDataKeyTLSCrt: []byte(*o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt),
			secretDataKeyTLSKey: []byte(*o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Key),
		}

		if err := o.runtimeClient.Client().Patch(ctx, s, client.MergeFrom(s)); err != nil {
			return fmt.Errorf("failed to update the secret reference in the runtime cluster (%s/%s) containing the TLS certificate for the Gardener Controller Manager", o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef.Namespace, o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef.Name)
		}
	}

	if o.imports.GardenerAdmissionController.Enabled && o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef != nil {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef.Name,
			Namespace: o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef.Namespace,
		}}

		s.Data = map[string][]byte{
			secretDataKeyTLSCrt: []byte(*o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt),
			secretDataKeyTLSKey: []byte(*o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Key),
		}

		if err := o.runtimeClient.Client().Patch(ctx, s, client.MergeFrom(s)); err != nil {
			return fmt.Errorf("failed to update the secret reference in the runtime cluster (%s/%s) containing the TLS certificate for the Gardener Admission Controller", o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef.Namespace, o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef.Name)
		}
	}

	return nil
}
