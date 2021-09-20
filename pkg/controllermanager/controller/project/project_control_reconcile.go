// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package project

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component/projectrbac"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *projectReconciler) reconcile(ctx context.Context, project *gardencorev1beta1.Project, gardenClient client.Client, gardenReader client.Reader) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(project, gardencorev1beta1.GardenerName) {
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient, project, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not add finalizer to Project: %w", err)
		}
	}

	// Ensure that we really get the latest version of the project to prevent working with an outdated version that has
	// an unset .spec.namespace field (which would result in trying to create another namespace again).
	if err := gardenReader.Get(ctx, kutil.Key(project.Name), project); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// If the project has no phase yet then we update it to be 'pending'.
	if len(project.Status.Phase) == 0 {
		if err := updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectPending }); err != nil {
			return reconcile.Result{}, err
		}
	}

	ownerReference := metav1.NewControllerRef(project, gardencorev1beta1.SchemeGroupVersion.WithKind("Project"))

	// We reconcile the namespace for the project: If the .spec.namespace is set then we try to claim it, if it is not
	// set then we create a new namespace with a random hash value.
	namespace, err := r.reconcileNamespaceForProject(ctx, gardenClient, project, ownerReference)
	if err != nil {
		r.recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, err.Error())
		_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectFailed })
		return reconcile.Result{}, err
	}
	r.reportEvent(project, false, gardencorev1beta1.ProjectEventNamespaceReconcileSuccessful, "Successfully reconciled namespace %q for project %q", namespace.Name, project.Name)

	// Update the name of the created namespace in the projects '.spec.namespace' field.
	if ns := project.Spec.Namespace; ns == nil {
		project.Spec.Namespace = &namespace.Name
		if err := gardenClient.Update(ctx, project); err != nil {
			r.reportEvent(project, false, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, err.Error())
			_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectFailed })

			// If we failed to update the namespace in the project specification we should try to delete
			// our created namespace again to prevent an inconsistent state.
			// TODO: this mechanism is only a best effort implementation, is fragile and can create orphaned namespaces.
			//  We should think about a better way to prevent an inconsistent state.
			if err := retryutils.UntilTimeout(ctx, time.Second, time.Minute, func(context.Context) (done bool, err error) {
				if err := gardenClient.Delete(ctx, namespace, kubernetes.DefaultDeleteOptions...); err != nil {
					if apierrors.IsNotFound(err) {
						return retryutils.Ok()
					}
					return retryutils.SevereError(err)
				}

				return retryutils.MinorError(fmt.Errorf("namespace %q still exists", namespace.Name))
			}); err != nil {
				r.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Failed to delete created namespace for project %q: %v", namespace.Name, err)
			}

			return reconcile.Result{}, err
		}
	}

	// Create RBAC rules to allow project members to interact with it.
	rbac, err := projectrbac.New(gardenClient, project)
	if err != nil {
		r.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while preparing for reconciling RBAC resources for namespace %q: %+v", namespace.Name, err)
		_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectFailed })
		return reconcile.Result{}, err
	}

	if err := rbac.Deploy(ctx); err != nil {
		r.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while reconciling RBAC resources for namespace %q: %+v", namespace.Name, err)
		_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectFailed })
		return reconcile.Result{}, err
	}

	if err := rbac.DeleteStaleExtensionRolesResources(ctx); err != nil {
		r.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while deleting stale RBAC rules for extension roles: %+v", err)
		_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectFailed })
		return reconcile.Result{}, err
	}

	// Create ResourceQuota for project if configured.
	quotaConfig, err := quotaConfiguration(r.config, project)
	if err != nil {
		r.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while setting up ResourceQuota: %+v", err)
		_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectFailed })
		return reconcile.Result{}, err
	}

	if quotaConfig != nil {
		if err := createOrUpdateResourceQuota(ctx, gardenClient, namespace.Name, ownerReference, *quotaConfig); err != nil {
			r.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while setting up ResourceQuota: %+v", err)
			_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectFailed })
			return reconcile.Result{}, err
		}
	}

	// Update the project status to mark it as 'ready'.
	if err := updateStatus(ctx, gardenClient, project, func() {
		project.Status.Phase = gardencorev1beta1.ProjectReady
		project.Status.ObservedGeneration = project.Generation
	}); err != nil {
		r.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while trying to mark project as ready: %+v", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *projectReconciler) reconcileNamespaceForProject(ctx context.Context, gardenClient client.Client, project *gardencorev1beta1.Project, ownerReference *metav1.OwnerReference) (*corev1.Namespace, error) {
	var (
		namespaceName = project.Spec.Namespace

		projectLabels      = namespaceLabelsFromProject(project)
		projectAnnotations = namespaceAnnotationsFromProject(project)
	)

	if namespaceName == nil {
		obj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName:    fmt.Sprintf("%s%s-", gutil.ProjectNamespacePrefix, project.Name),
				OwnerReferences: []metav1.OwnerReference{*ownerReference},
				Labels:          projectLabels,
				Annotations:     projectAnnotations,
			},
		}
		return obj, gardenClient.Create(ctx, obj)
	}

	namespace := &corev1.Namespace{}
	if err := gardenClient.Get(ctx, client.ObjectKey{Name: *namespaceName}, namespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		obj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:            *namespaceName,
				OwnerReferences: []metav1.OwnerReference{*ownerReference},
				Labels:          projectLabels,
				Annotations:     projectAnnotations,
			},
		}
		return obj, gardenClient.Create(ctx, obj)
	}

	if !apiequality.Semantic.DeepDerivative(projectLabels, namespace.Labels) {
		return nil, fmt.Errorf("namespace cannot be used as it needs the project labels %#v", projectLabels)
	}

	if metav1.HasAnnotation(namespace.ObjectMeta, v1beta1constants.NamespaceProject) && !apiequality.Semantic.DeepDerivative(projectAnnotations, namespace.Annotations) {
		return nil, fmt.Errorf("namespace is already in-use by another project")
	}

	before := namespace.DeepCopy()

	namespace.OwnerReferences = kutil.MergeOwnerReferences(namespace.OwnerReferences, *ownerReference)
	namespace.Labels = utils.MergeStringMaps(namespace.Labels, projectLabels)
	namespace.Annotations = utils.MergeStringMaps(namespace.Annotations, projectAnnotations)

	// If the project is reconciled for the first time then its observed generation is 0. Only in this case we want
	// to add the "keep-after-project-deletion" annotation to the namespace when we adopt it.
	if project.Status.ObservedGeneration == 0 {
		namespace.Annotations[v1beta1constants.NamespaceKeepAfterProjectDeletion] = "true"
	}

	if apiequality.Semantic.DeepEqual(before, namespace) {
		return namespace, nil
	}

	// update namespace if required
	return namespace, gardenClient.Update(ctx, namespace)
}

// quotaConfiguration returns the first matching quota configuration if one is configured for the given project.
func quotaConfiguration(config *config.ProjectControllerConfiguration, project *gardencorev1beta1.Project) (*config.QuotaConfiguration, error) {
	if config == nil {
		return nil, nil
	}

	for _, c := range config.Quotas {
		quotaConfig := c
		selector, err := metav1.LabelSelectorAsSelector(quotaConfig.ProjectSelector)
		if err != nil {
			return nil, err
		}

		if selector.Matches(labels.Set(project.GetLabels())) {
			return &quotaConfig, nil
		}
	}

	return nil, nil
}

// ResourceQuotaName is the name of the default ResourceQuota resource that is created by Gardener in the project namespace.
const ResourceQuotaName = "gardener"

func createOrUpdateResourceQuota(ctx context.Context, c client.Client, projectNamespace string, ownerReference *metav1.OwnerReference, config config.QuotaConfiguration) error {
	resourceQuota, ok := config.Config.(*corev1.ResourceQuota)
	if !ok {
		return fmt.Errorf("failure while reading ResourceQuota from configuration: %v", resourceQuota)
	}

	projectResourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ResourceQuotaName,
			Namespace: projectNamespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c, projectResourceQuota, func() error {
		projectResourceQuota.SetOwnerReferences(kutil.MergeOwnerReferences(projectResourceQuota.GetOwnerReferences(), *ownerReference))
		projectResourceQuota.Labels = utils.MergeStringMaps(projectResourceQuota.Labels, resourceQuota.Labels)
		projectResourceQuota.Annotations = utils.MergeStringMaps(projectResourceQuota.Annotations, resourceQuota.Annotations)
		quotas := make(map[corev1.ResourceName]resource.Quantity)
		for resourceName, quantity := range resourceQuota.Spec.Hard {
			if val, ok := projectResourceQuota.Spec.Hard[resourceName]; ok {
				// Do not overwrite already existing quotas.
				quotas[resourceName] = val
				continue
			}
			quotas[resourceName] = quantity
		}
		projectResourceQuota.Spec.Hard = quotas
		return nil
	}); err != nil {
		return err
	}

	return nil
}
