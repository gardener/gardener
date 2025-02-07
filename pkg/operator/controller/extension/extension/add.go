// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/admission"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/controllerregistration"
	extensionruntime "github.com/gardener/gardener/pkg/operator/controller/extension/extension/runtime"
	operatorpredicate "github.com/gardener/gardener/pkg/operator/predicate"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// ControllerName is the name of this controller.
const ControllerName = "extension"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	var err error

	if r.RuntimeClientSet == nil {
		r.RuntimeClientSet, err = kubernetes.NewWithConfig(
			kubernetes.WithRESTConfig(mgr.GetConfig()),
			kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
			kubernetes.WithRuntimeClient(mgr.GetClient()),
			kubernetes.WithRuntimeCache(mgr.GetCache()),
		)
		if err != nil {
			return fmt.Errorf("failed creating runtime clientset: %w", err)
		}
	}

	if r.GardenClientMap == nil {
		return fmt.Errorf("GardenClientMap must not be nil")
	}

	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")
	}

	if r.HelmRegistry == nil {
		r.HelmRegistry, err = oci.NewHelmRegistry(r.RuntimeClientSet.Client())
		if err != nil {
			return fmt.Errorf("failed creating Helm registry: %w", err)
		}
	}

	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	r.admission = admission.New(r.RuntimeClientSet, r.Recorder, r.GardenNamespace, r.HelmRegistry)
	r.controllerRegistration = controllerregistration.New(r.RuntimeClientSet.Client(), r.Recorder, r.GardenNamespace)
	r.runtime = extensionruntime.New(r.RuntimeClientSet, r.Recorder, r.GardenNamespace, r.HelmRegistry)

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&operatorv1alpha1.Extension{}, builder.WithPredicates(predicate.Or(
			predicate.GenerationChangedPredicate{},
			operatorpredicate.ExtensionRequirementsChanged(),
		))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.Extension.ConcurrentSyncs, 0),
		}).
		Watches(
			&operatorv1alpha1.Garden{},
			handler.EnqueueRequestsFromMapFunc(r.MapToAllExtensions(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(predicate.Or(
				operatorpredicate.GardenCreatedOrReconciledSuccessfully(),
				operatorpredicate.GardenDeletionTriggered(),
			)),
		).
		Complete(r)
}

// MapToAllExtensions returns reconcile.Request objects for all existing gardens in the system.
func (r *Reconciler) MapToAllExtensions(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		extensionList := &metav1.PartialObjectMetadataList{}
		extensionList.SetGroupVersionKind(operatorv1alpha1.SchemeGroupVersion.WithKind("ExtensionList"))
		if err := r.RuntimeClientSet.Client().List(ctx, extensionList); err != nil {
			log.Error(err, "Failed to list extensions")
			return nil
		}

		return mapper.ObjectListToRequests(extensionList)
	}
}
