// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mapper

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// MapControllerInstallationToExtension returns a mapper that maps the ControllerInstallation to the Extension object.
func MapControllerInstallationToExtension(runtimeClient client.Client, log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
		if !ok {
			return nil
		}

		var (
			extensionName = controllerInstallation.Spec.RegistrationRef.Name
			extension     = &operatorv1alpha1.Extension{}
		)

		if err := runtimeClient.Get(ctx, client.ObjectKey{Name: controllerInstallation.Spec.RegistrationRef.Name}, extension); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			log.Error(err, "Unable to get extension", "extension", extensionName)
		}

		return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(extension)}}
	}
}
