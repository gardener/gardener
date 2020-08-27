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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *defaultControl) delete(ctx context.Context, project *gardencorev1beta1.Project) (bool, error) {
	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return false, fmt.Errorf("failed to get garden client: %w", err)
	}

	if namespace := project.Spec.Namespace; namespace != nil {
		released, err := c.releaseNamespace(ctx, gardenClient, project, *namespace)
		if err != nil {
			c.reportEvent(project, true, gardencorev1beta1.ProjectEventNamespaceDeletionFailed, err.Error())
			_, _ = updateProjectStatus(ctx, gardenClient.GardenCore(), project.ObjectMeta, setProjectPhase(gardencorev1beta1.ProjectFailed))
			return false, err
		}

		if !released {
			c.reportEvent(project, false, gardencorev1beta1.ProjectEventNamespaceMarkedForDeletion, "Successfully marked namespace %q for deletion.", *namespace)
			_, _ = updateProjectStatus(ctx, gardenClient.GardenCore(), project.ObjectMeta, setProjectPhase(gardencorev1beta1.ProjectTerminating))
			return true, nil
		}
	}

	return false, controllerutils.RemoveFinalizer(ctx, gardenClient.DirectClient(), project, gardencorev1beta1.GardenerName)
}

func (c *defaultControl) releaseNamespace(ctx context.Context, gardenClient kubernetes.Interface, project *gardencorev1beta1.Project, namespaceName string) (bool, error) {
	namespace, err := c.namespaceLister.Get(namespaceName)
	if err != nil {
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
	if !apiequality.Semantic.DeepDerivative(namespaceLabelsFromProjectDeprecated(project), namespace.Labels) {
		return true, nil
	}

	// If the user wants to keep the namespace in the system even if the project gets deleted then we remove the related
	// labels, annotations, and owner references and only delete the project.
	var keepNamespace bool
	if val, ok := namespace.Annotations[common.NamespaceKeepAfterProjectDeletion]; ok {
		keepNamespace, _ = strconv.ParseBool(val)
	}

	if keepNamespace {
		delete(namespace.Annotations, common.NamespaceProject)
		delete(namespace.Annotations, common.NamespaceProjectDeprecated)
		delete(namespace.Annotations, common.NamespaceKeepAfterProjectDeletion)
		delete(namespace.Labels, common.ProjectName)
		delete(namespace.Labels, v1beta1constants.GardenRole)
		delete(namespace.Labels, common.ProjectNameDeprecated)
		delete(namespace.Labels, v1beta1constants.DeprecatedGardenRole)
		for i := len(namespace.OwnerReferences) - 1; i >= 0; i-- {
			if ownerRef := namespace.OwnerReferences[i]; ownerRef.APIVersion == gardencorev1beta1.SchemeGroupVersion.String() &&
				ownerRef.Kind == "Project" &&
				ownerRef.Name == project.Name &&
				ownerRef.UID == project.UID {
				namespace.OwnerReferences = append(namespace.OwnerReferences[:i], namespace.OwnerReferences[i+1:]...)
			}
		}
		err = gardenClient.Client().Update(ctx, namespace)
		return true, err
	}

	err = gardenClient.Client().Delete(ctx, namespace, kubernetes.DefaultDeleteOptions...)
	return false, client.IgnoreNotFound(err)
}
