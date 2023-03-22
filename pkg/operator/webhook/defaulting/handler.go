// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package defaulting

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// Handler performs defaulting.
type Handler struct {
	Logger logr.Logger
}

// Default performs the defaulting.
func (h *Handler) Default(_ context.Context, obj runtime.Object) error {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", obj)
	}

	if kubeAPIServer := garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; kubeAPIServer != nil {
		kubeAPIServer.KubeAPIServerConfig = &gardencorev1beta1.KubeAPIServerConfig{}
		gardencorev1beta1.SetDefaults_KubeAPIServerConfig(kubeAPIServer.KubeAPIServerConfig)
	}

	return nil
}
