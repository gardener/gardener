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

func (r *projectReconciler) delete(ctx context.Context, project *gardencorev1beta1.Project, gardenClient client.Client, gardenReader client.Reader) (reconcile.Result, error) {
	if namespace := project.Spec.Namespace; namespace != nil {
		inUse, err := kutil.IsNamespaceInUse(ctx, gardenReader, *namespace, gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to check if namespace is empty: %w", err)
		}

		if inUse {
			r.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceNotEmpty, "Cannot release namespace %q because it still contains Shoots.", *namespace)

			_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectTerminating })
			return reconcile.Result{RequeueAfter: time.Minute}, nil
		}

		released, err := r.releaseNamespace(ctx, gardenClient, project, *namespace)
		if err != nil {
			r.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceDeletionFailed, err.Error())
			_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectFailed })
			return reconcile.Result{}, err
		}

		if !released {
			r.reportEvent(project, false, gardencorev1beta1.ProjectEventNamespaceMarkedForDeletion, "Successfully marked namespace %q for deletion.", *namespace)
			_ = updateStatus(ctx, gardenClient, project, func() { project.Status.Phase = gardencorev1beta1.ProjectTerminating })
			return reconcile.Result{RequeueAfter: time.Minute}, nil
		}
	}

	return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, gardenClient, project, gardencorev1beta1.GardenerName)
}

func (r *projectReconciler) releaseNamespace(ctx context.Context, gardenClient client.Client, project *gardencorev1beta1.Project, namespaceName string) (bool, error) {
	namespace := &corev1.Namespace{}
	if err := r.gardenClient.Client().Get(ctx, kutil.Key(namespaceName), namespace); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	// If the namespace has been already marked for deletion we do not need to do it again.
	if namespace.DeletionTimestamp != nil {
		return false, nil
	}

	// To prevent "stealing" namespaces by other projects we only delete the namespace if its labels match
	// the project labels.
	if !apiequality.Semantic.DeepDerivative(namespaceLabelsFromProject(project), namespace.Labels) {
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
		err := gardenClient.Update(ctx, namespace)
		return true, err
	}

	err := gardenClient.Delete(ctx, namespace, kubernetes.DefaultDeleteOptions...)
	return false, client.IgnoreNotFound(err)
}
