// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensionclusterrole

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
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
			Key:      v1beta1constants.LabelExtensionsAuthorizationAdditionalPermissions,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"true"},
		}},
	})
	utilruntime.Must(err)
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	clusterRole := &metav1.PartialObjectMetadata{}
	clusterRole.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"))

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(clusterRole, builder.WithPredicates(labelSelectorPredicate)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		Build(r)
	if err != nil {
		return err
	}

	serviceAccount := &metav1.PartialObjectMetadata{}
	serviceAccount.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceAccount"))

	return c.Watch(
		source.Kind(mgr.GetCache(), serviceAccount),
		mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapToAllClusterRoles), mapper.UpdateWithNew, c.GetLogger()),
		r.ServiceAccountPredicate(),
	)
}

// ServiceAccountPredicate returns true when the name of the ServiceAccount is prefixed with `extension-` and when its
// namespace is prefixed with `seed-`.
func (r *Reconciler) ServiceAccountPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return strings.HasPrefix(obj.GetName(), v1beta1constants.ExtensionGardenServiceAccountPrefix) &&
			strings.HasPrefix(obj.GetNamespace(), gardenerutils.SeedNamespaceNamePrefix)
	})
}

// MapToAllClusterRoles returns reconcile.Request objects for all ClusterRoles with a respective label.
func (r *Reconciler) MapToAllClusterRoles(ctx context.Context, log logr.Logger, reader client.Reader, _ client.Object) []reconcile.Request {
	clusterRoleList := &metav1.PartialObjectMetadataList{}
	clusterRoleList.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleList"))
	if err := reader.List(ctx, clusterRoleList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.LabelExtensionsAuthorizationAdditionalPermissions, selection.In, "true"))}); err != nil {
		log.Error(err, "Failed to list ClusterRoles")
		return nil
	}

	return mapper.ObjectListToRequests(clusterRoleList)
}
