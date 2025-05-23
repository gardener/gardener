// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa

import (
	"context"
	"errors"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootVPAEnabledByDefault, func(_ io.Reader) (admission.Interface, error) {
		return New(), nil
	})
}

// ShootVPA contains required information to process admission requests.
type ShootVPA struct {
	*admission.Handler
}

// New creates a new ShootVPA admission plugin.
func New() admission.MutationInterface {
	return &ShootVPA{
		Handler: admission.NewHandler(admission.Create),
	}
}

// Admit defaults spec.kubernetes.verticalPodAutoscaler.enabled=true for new shoot clusters.
func (c *ShootVPA) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	switch {
	case a.GetKind().GroupKind() != core.Kind("Shoot"),
		a.GetOperation() != admission.Create,
		a.GetSubresource() != "":
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	if shoot.Spec.Kubernetes.VerticalPodAutoscaler == nil && !gardencorehelper.IsWorkerless(shoot) {
		shoot.Spec.Kubernetes.VerticalPodAutoscaler = &core.VerticalPodAutoscaler{Enabled: true}
	}

	return nil
}
