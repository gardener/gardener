// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
)

// ProjectForNamespaceFromLister returns the Project responsible for a given <namespace>. It lists all Projects
// via the given lister, iterates over them and tries to identify the Project by looking for the namespace name
// in the project spec.
func ProjectForNamespaceFromLister(projectLister gardencorev1beta1listers.ProjectLister, namespaceName string) (*gardencorev1beta1.Project, error) {
	projectList, err := projectLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, project := range projectList {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == namespaceName {
			return project, nil
		}
	}

	return nil, apierrors.NewNotFound(core.Resource("Project"), "<unknown>")
}
