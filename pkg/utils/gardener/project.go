// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
)

// ProjectNamespacePrefix is the prefix of namespaces representing projects.
const ProjectNamespacePrefix = "garden-"

// ProjectForNamespaceFromReader returns the Project responsible for a given <namespace>. It reads the namespace and
// fetches the project name label. Then it will read the project with the respective name.
func ProjectForNamespaceFromReader(ctx context.Context, reader client.Reader, namespaceName string) (*gardencorev1beta1.Project, error) {
	projectList := &gardencorev1beta1.ProjectList{}
	if err := reader.List(ctx, projectList, client.MatchingFields{gardencore.ProjectNamespace: namespaceName}); err != nil {
		return nil, err
	}

	if len(projectList.Items) == 0 {
		return nil, apierrors.NewNotFound(gardencorev1beta1.Resource("Project"), "<unknown>")
	}

	return &projectList.Items[0], nil
}

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

	return nil, apierrors.NewNotFound(gardencore.Resource("Project"), "<unknown>")
}

// ProjectAndNamespaceFromReader returns the Project responsible for a given <namespace>. It reads the namespace and
// fetches the project name label. Then it will read the project with the respective name.
func ProjectAndNamespaceFromReader(ctx context.Context, reader client.Reader, namespaceName string) (*gardencorev1beta1.Project, *corev1.Namespace, error) {
	namespace := &corev1.Namespace{}
	if err := reader.Get(ctx, client.ObjectKey{Name: namespaceName}, namespace); err != nil {
		return nil, nil, err
	}

	projectName := namespace.Labels[v1beta1constants.ProjectName]
	if projectName == "" {
		return nil, namespace, nil
	}

	project := &gardencorev1beta1.Project{}
	if err := reader.Get(ctx, client.ObjectKey{Name: projectName}, project); err != nil {
		return nil, namespace, err
	}

	return project, namespace, nil
}
