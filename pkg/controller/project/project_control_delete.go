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
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/sirupsen/logrus"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (c *defaultControl) delete(project *gardenv1beta1.Project, projectLogger logrus.FieldLogger) (bool, error) {
	if namespace := project.Spec.Namespace; namespace != nil {
		alreadyDeleted, err := c.deleteNamespace(project, *namespace)
		if err != nil {
			c.reportEvent(project, true, gardenv1beta1.ProjectEventNamespaceDeletionFailed, err.Error())
			c.updateProjectStatus(project.ObjectMeta, setProjectPhase(gardenv1beta1.ProjectFailed))
			return false, err
		}

		if !alreadyDeleted {
			c.reportEvent(project, false, gardenv1beta1.ProjectEventNamespaceMarkedForDeletion, "Successfully marked namespace %q for deletion.", *namespace)
			c.updateProjectStatus(project.ObjectMeta, setProjectPhase(gardenv1beta1.ProjectTerminating))
			return true, nil
		}
	}

	// Remove finalizer from project resource.
	projectFinalizers := sets.NewString(project.Finalizers...)
	projectFinalizers.Delete(gardenv1beta1.GardenerName)
	project.Finalizers = projectFinalizers.UnsortedList()
	if _, err := c.k8sGardenClient.GardenClientset().GardenV1beta1().Projects().Update(project); err != nil && !apierrors.IsNotFound(err) {
		projectLogger.Error(err.Error())
		return false, err
	}
	return false, nil
}

func (c *defaultControl) deleteNamespace(project *gardenv1beta1.Project, namespaceName string) (bool, error) {
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
	if !apiequality.Semantic.DeepDerivative(namespaceLabelsFromProject(project), namespace.Labels) {
		return true, nil
	}

	if err := c.k8sGardenClient.DeleteNamespace(namespaceName); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	return false, nil
}
