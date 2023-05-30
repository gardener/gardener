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

package kubernetesservicehost

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles admission requests and injects the KUBERNETES_SERVICE_HOST environment variable into all containers
// in pods.
type Handler struct {
	Logger logr.Logger
	Host   string
}

// Default injects the KUBERNETES_SERVICE_HOST environment variable into all containers in the pod.
func (h *Handler) Default(ctx context.Context, obj runtime.Object) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("expected *corev1.Pod but got %T", obj)
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	log := h.Logger.WithValues("pod", kubernetesutils.ObjectKeyForCreateWebhooks(pod, req))

	log.Info("Injecting KUBERNETES_SERVICE_HOST environment variable into all containers in the pod")
	h.mutateContainers(pod.Spec.InitContainers)
	h.mutateContainers(pod.Spec.Containers)

	return nil
}

func (h *Handler) mutateContainers(containers []corev1.Container) {
	for i, container := range containers {
		if hasEnv(container.Env) {
			continue
		}

		if container.Env == nil {
			container.Env = make([]corev1.EnvVar, 0, 1)
		}

		containers[i].Env = append(containers[i].Env, corev1.EnvVar{
			Name:      "KUBERNETES_SERVICE_HOST",
			Value:     h.Host,
			ValueFrom: nil,
		})
	}
}

func hasEnv(envVars []corev1.EnvVar) bool {
	for _, env := range envVars {
		if env.Name == "KUBERNETES_SERVICE_HOST" {
			return true
		}
	}
	return false
}
