//  SPDX-FileCopyrightText: Contributors to the Gardener project
//
//  SPDX-License-Identifier: Apache-2.0

package predicate

import (
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GardenWildcardCertificateChanged is a predicate which returns 'true' if a secret with the wildcard certificate role changed.
func GardenWildcardCertificateChanged() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return secretIsGardenWildcardCertificate(e.Object.(*corev1.Secret))
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			secret, ok := e.ObjectNew.(*corev1.Secret)
			if !ok {
				return false
			}

			oldSecret, ok := e.ObjectOld.(*corev1.Secret)
			if !ok {
				return false
			}

			if secretIsGardenWildcardCertificate(oldSecret) || secretIsGardenWildcardCertificate(secret) {
				return !apiequality.Semantic.DeepEqual(secret.Data, oldSecret.Data) ||
					oldSecret.Labels[v1beta1constants.GardenRole] != secret.Labels[v1beta1constants.GardenRole]
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return secretIsGardenWildcardCertificate(e.Object.(*corev1.Secret))
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func secretIsGardenWildcardCertificate(secret *corev1.Secret) bool {
	return secret.Labels[v1beta1constants.GardenRole] == v1beta1constants.GardenRoleGardenWildcardCert
}
