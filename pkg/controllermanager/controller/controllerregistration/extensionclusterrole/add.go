// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionclusterrole

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerregistration-extension-clusterrole"

var labelSelectorPredicate predicate.Predicate

func init() {
	var err error
	labelSelectorPredicate, err = predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      v1beta1constants.LabelAuthorizationCustomExtensionsPermissions,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"true"},
		}},
	})
	utilruntime.Must(err)
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	clusterRole := &metav1.PartialObjectMetadata{}
	clusterRole.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"))

	serviceAccount := &metav1.PartialObjectMetadata{}
	serviceAccount.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceAccount"))

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(clusterRole, builder.WithPredicates(labelSelectorPredicate)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		Watches(
			serviceAccount,
			handler.EnqueueRequestsFromMapFunc(r.MapToMatchingClusterRoles(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(r.ServiceAccountPredicate()),
		).
		Complete(r)
}

// ServiceAccountPredicate returns true when the namespace is prefixed with `seed-`.
func (r *Reconciler) ServiceAccountPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return strings.HasPrefix(obj.GetNamespace(), gardenerutils.SeedNamespaceNamePrefix)
	})
}

// MapToMatchingClusterRoles returns reconcile.Request objects for all ClusterRoles whose service account selector
// matches the labels of the given service account object.
func (r *Reconciler) MapToMatchingClusterRoles(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, serviceAccount client.Object) []reconcile.Request {
		clusterRoleList := &metav1.PartialObjectMetadataList{}
		clusterRoleList.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleList"))
		if err := r.Client.List(ctx, clusterRoleList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.LabelAuthorizationCustomExtensionsPermissions, selection.In, "true"))}); err != nil {
			log.Error(err, "Failed to list ClusterRoles")
			return nil
		}

		var requests []reconcile.Request
		for _, clusterRole := range clusterRoleList.Items {
			labelSelector, err := clusterRoleServiceAccountLabelSelectorToSelector(clusterRole.Annotations)
			if err != nil {
				log.Error(err, "Failed parsing label selector", "clusterRoleName", clusterRole.Name)
				continue
			}

			if labelSelector.Matches(labels.Set(serviceAccount.GetLabels())) {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterRole.Name}})
			}
		}

		return requests
	}
}

func clusterRoleServiceAccountLabelSelectorToSelector(annotations map[string]string) (labels.Selector, error) {
	selectorJSON, ok := annotations[v1beta1constants.LabelAuthorizationExtensionsServiceAccountSelector]
	if !ok {
		return nil, fmt.Errorf("no service account selector annotations present")
	}

	var selector metav1.LabelSelector
	if err := json.Unmarshal([]byte(selectorJSON), &selector); err != nil {
		return nil, fmt.Errorf("failed unmarshalling extensions service account selector %s: %w", selectorJSON, err)
	}

	labelSelector, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil {
		return nil, fmt.Errorf("failed parsing label selector %s: %w", selectorJSON, err)
	}

	return labelSelector, nil
}
