// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubeconfigsecret

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
func (h *Handler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return h.handle(ctx, obj)
}

// ValidateUpdate performs the check.
func (h *Handler) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return h.handle(ctx, newObj)
}

// ValidateDelete returns nil (not implemented by this handler).
func (h *Handler) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (h *Handler) handle(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", obj))
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	kubeconfig, ok := secret.Data[kubernetes.KubeConfig]
	if !ok {
		return nil, nil
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, apierrors.NewBadRequest(err.Error())
	}

	// Validate that the given kubeconfig doesn't have fields in its auth-info that are
	// not acceptable.
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, apierrors.NewBadRequest(err.Error())
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

		return nil, apierrors.NewInvalid(schema.GroupKind{Group: corev1.GroupName, Kind: "Secret"}, secret.Name, field.ErrorList{field.Invalid(field.NewPath("data", "kubeconfig"), kubeconfig, fmt.Sprintf("secret contains invalid kubeconfig: %s", err))})
	}

	return nil, nil
}
