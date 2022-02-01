// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"context"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// ProjectAndNamespaceFromReader returns the Project responsible for a given <namespace>. It reads the namespace and
// fetches the project name label. Then it will read the project with the respective name.
func ProjectAndNamespaceFromReader(ctx context.Context, reader client.Reader, namespaceName string) (*gardencorev1beta1.Project, *corev1.Namespace, error) {
	namespace := &corev1.Namespace{}
	if err := reader.Get(ctx, kutil.Key(namespaceName), namespace); err != nil {
		return nil, nil, err
	}

	projectName := namespace.Labels[v1beta1constants.ProjectName]
	if projectName == "" {
		return nil, namespace, nil
	}

	project := &gardencorev1beta1.Project{}
	if err := reader.Get(ctx, kutil.Key(projectName), project); err != nil {
		return nil, namespace, err
	}

	return project, namespace, nil
}
