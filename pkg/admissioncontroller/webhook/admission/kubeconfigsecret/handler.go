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

package kubeconfigsecret

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/metrics"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// metricReasonRejectedKubeconfig is a metric reason value for a reason when a kubeconfig was rejected.
const metricReasonRejectedKubeconfig = "Rejected Kubeconfig"

// Handler checks, if the secrets contains a kubeconfig and denies kubeconfigs with invalid fields (e.g. tokenFile or
// exec).
type Handler struct {
	Logger logr.Logger
}

// ValidateCreate performs the check.
func (h *Handler) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return h.handle(ctx, obj)
}

// ValidateUpdate performs the check.
func (h *Handler) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) error {
	return h.handle(ctx, newObj)
}

// ValidateDelete returns nil (not implemented by this handler).
func (h *Handler) ValidateDelete(_ context.Context, _ runtime.Object) error {
	return nil
}

func (h *Handler) handle(ctx context.Context, obj runtime.Object) error {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("expected *corev1.Secret but got %T", obj)
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	kubeconfig, ok := secret.Data[kubernetes.KubeConfig]
	if !ok {
		return nil
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return err
	}

	// Validate that the given kubeconfig doesn't have fields in its auth-info that are
	// not acceptable.
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return err
	}

	if err := kubernetes.ValidateConfig(rawConfig); err != nil {
		h.Logger.Info("Rejected secret",
			"namespace", secret.Namespace,
			"name", secret.Name,
			"username", req.UserInfo.Username,
			"reason", err.Error(),
		)

		metrics.RejectedResources.WithLabelValues(
			string(req.Operation),
			req.Kind.Kind,
			req.Namespace,
			metricReasonRejectedKubeconfig,
		).Inc()

		return fmt.Errorf("secret contains invalid kubeconfig: %w", err)
	}

	return nil
}
