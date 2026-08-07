// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagepullsecret

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles admission requests and injects image pull secrets into pods based on the configured image vector.
type Handler struct {
	Logger logr.Logger
}

// Default injects image pull secrets into the pod based on the container images and the configured image vector.
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

	log.Info("Injecting image pull secrets into the pod")
	kubernetesutils.InjectImagePullSecret(&pod.Spec)

	return nil
}
