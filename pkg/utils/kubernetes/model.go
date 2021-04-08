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

package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GetObjectMetaModel returns the model of the given metav1.ObjectMeta.
func GetObjectMetaModel(meta *metav1.ObjectMeta) *metav1.ObjectMeta {
	model := &metav1.ObjectMeta{
		Name:            meta.Name,
		Namespace:       meta.Namespace,
		Labels:          meta.Labels,
		Annotations:     meta.Annotations,
		OwnerReferences: meta.OwnerReferences,
	}
	delete(model.Annotations, v1beta1constants.AnnotationModelChecksum)
	return model
}

// GetSecretModel returns the model of the given corev1.Secret.
func GetSecretModel(secret *corev1.Secret) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: *GetObjectMetaModel(&secret.ObjectMeta),
		Data:       secret.Data,
		StringData: secret.StringData,
		Type:       secret.Type,
	}
}
