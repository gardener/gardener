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
	"strconv"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (r *projectReconciler) delete(ctx context.Context, log logr.Logger, project *gardencorev1beta1.Project, gardenClient client.Client) (reconcile.Result, error) {
	if namespace := project.Spec.Namespace; namespace != nil {
		log = log.WithValues("namespaceName", *namespace)

		inUse, err := kutil.IsNamespaceInUse(ctx, gardenClient, *namespace, gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to check if namespace is empty: %w", err)
		}

		if inUse {
			r.recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceNotEmpty, "Cannot release namespace %q because it still contains Shoots", *namespace)
			log.Info("Cannot release Project Namespace because it still contains Shoots")

			_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectTerminating })
			// requeue with exponential backoff
			return reconcile.Result{Requeue: true}, nil
		}

		released, err := r.releaseNamespace(ctx, log, gardenClient, project, *namespace)
		if err != nil {
			r.recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceDeletionFailed, "Failed to release project namespace %q: %v", *namespace, err)
			_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectFailed })
			return reconcile.Result{}, fmt.Errorf("failed to release project namespace: %w", err)
		}

		if !released {
			r.recorder.Eventf(project, corev1.EventTypeNormal, gardencorev1beta1.ProjectEventNamespaceMarkedForDeletion, "Successfully marked project namespace %q for deletion", *namespace)
			_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectTerminating })
			return reconcile.Result{RequeueAfter: time.Minute}, nil
		}
	}

	return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, gardenClient, project, gardencorev1beta1.GardenerName)
}

func (r *projectReconciler) releaseNamespace(ctx context.Context, log logr.Logger, gardenClient client.Client, project *gardencorev1beta1.Project, namespaceName string) (bool, error) {
	namespace := &corev1.Namespace{}
	if err := r.gardenClient.Client().Get(ctx, kutil.Key(namespaceName), namespace); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	// If the namespace has been already marked for deletion we do not need to do it again.
	if namespace.DeletionTimestamp != nil {
		log.Info("Project Namespace is already marked for deletion, nothing to do for releasing it")
		return false, nil
	}

	// To prevent "stealing" namespaces by other projects we only delete the namespace if its labels match
	// the project labels.
	if !apiequality.Semantic.DeepDerivative(namespaceLabelsFromProject(project), namespace.Labels) {
		log.Info("Referenced Namespace does not belong to this Project, nothing to do for releasing it")
		return true, nil
	}

	// If the user wants to keep the namespace in the system even if the project gets deleted then we remove the related
	// labels, annotations, and owner references and only delete the project.
	var keepNamespace bool
	if val, ok := namespace.Annotations[v1beta1constants.NamespaceKeepAfterProjectDeletion]; ok {
		keepNamespace, _ = strconv.ParseBool(val)
	}

	if keepNamespace {
		delete(namespace.Annotations, v1beta1constants.NamespaceProject)
		delete(namespace.Annotations, v1beta1constants.NamespaceKeepAfterProjectDeletion)
		delete(namespace.Annotations, v1beta1constants.NamespaceCreatedByProjectController)
		delete(namespace.Labels, v1beta1constants.ProjectName)
		delete(namespace.Labels, v1beta1constants.GardenRole)
		for i := len(namespace.OwnerReferences) - 1; i >= 0; i-- {
			if ownerRef := namespace.OwnerReferences[i]; ownerRef.APIVersion == gardencorev1beta1.SchemeGroupVersion.String() &&
				ownerRef.Kind == "Project" &&
				ownerRef.Name == project.Name &&
				ownerRef.UID == project.UID {
				namespace.OwnerReferences = append(namespace.OwnerReferences[:i], namespace.OwnerReferences[i+1:]...)
			}
		}
		log.Info("Project Namespace should be kept, removing owner references")
		err := gardenClient.Update(ctx, namespace)
		return true, err
	}

	log.Info("Deleting Project Namespace")
	err := gardenClient.Delete(ctx, namespace, kubernetes.DefaultDeleteOptions...)
	return false, client.IgnoreNotFound(err)
}
