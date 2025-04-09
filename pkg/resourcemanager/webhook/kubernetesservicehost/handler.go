// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	kubernetesutils.InjectKubernetesServiceHostEnv(pod.Spec.InitContainers, h.Host)
	kubernetesutils.InjectKubernetesServiceHostEnv(pod.Spec.Containers, h.Host)

	return nil
}
