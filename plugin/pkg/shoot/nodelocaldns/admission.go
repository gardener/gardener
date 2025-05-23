// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodelocaldns

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
	plugins.Register(plugin.PluginNameShootNodeLocalDNSEnabledByDefault, func(_ io.Reader) (admission.Interface, error) {
		return New(), nil
	})
}

// ShootNodeLocalDNS contains required information to process admission requests.
type ShootNodeLocalDNS struct {
	*admission.Handler
}

// New creates a new ShootNodeLocalDNS admission plugin.
func New() admission.MutationInterface {
	return &ShootNodeLocalDNS{
		Handler: admission.NewHandler(admission.Create),
	}
}

// Admit defaults spec.systemComponents.nodeLocalDNS.enabled=true for new shoot clusters.
func (c *ShootNodeLocalDNS) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	if !gardencorehelper.IsWorkerless(shoot) {
		if shoot.Spec.SystemComponents == nil {
			shoot.Spec.SystemComponents = &core.SystemComponents{}
		}

		if shoot.Spec.SystemComponents.NodeLocalDNS == nil {
			shoot.Spec.SystemComponents.NodeLocalDNS = &core.NodeLocalDNS{Enabled: true}
		}
	}

	return nil
}
