// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootMutator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// MutateShoot is an implementation of admission.Interface.
type MutateShoot struct {
	*admission.Handler
}

// New creates a new MutateShoot admission plugin.
func New() (*MutateShoot, error) {
	return &MutateShoot{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

var _ admission.MutationInterface = (*MutateShoot)(nil)

// Admit mutates the Shoot.
func (m *MutateShoot) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert object to Shoot")
	}

	if a.GetOperation() == admission.Create {
		addCreatedByAnnotation(shoot, a.GetUserInfo().GetName())
	}

	return nil
}

func addCreatedByAnnotation(shoot *core.Shoot, userName string) {
	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenCreatedBy, userName)
}
