// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretnamechange

import (
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of this controller.
const ControllerName = "secret-name-change"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, nodePredicate predicate.Predicate) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.ConfigDir == "" {
		r.ConfigDir = nodeagentconfigv1alpha1.BaseDir
	}
	if r.FS.Fs == nil {
		r.FS = afero.Afero{Fs: afero.NewOsFs()}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Node{}, builder.WithPredicates(r.NodePredicate(), nodePredicate)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		Complete(r)
}

// NodePredicate returns 'true' when the label describing which gardener-node-agent secret is relevant for this
// node gets set or changed.
func (r *Reconciler) NodePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetLabels()[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName] != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetLabels()[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName] != e.ObjectNew.GetLabels()[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
