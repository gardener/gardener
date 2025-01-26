// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"io"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameProjectValidator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// This admission plugin was supposed to be removed in favor of static validation in a future release, see https://github.com/gardener/gardener/pull/4228.
// However, it cannot be removed, because the static validation cannot differ `CREATE` from `UPDATE` operation.
type handler struct {
	*admission.Handler
}

// New creates a new handler admission plugin.
func New() (*handler, error) {
	return &handler{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

var _ admission.ValidationInterface = &handler{}

func (v *handler) Validate(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Ignore all kinds other than Project
	if a.GetKind().GroupKind() != gardencore.Kind("Project") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	// Convert object to Project
	project, ok := a.GetObject().(*gardencore.Project)
	if !ok {
		return apierrors.NewBadRequest("could not convert object to Project")
	}

	if project.Spec.Namespace != nil && *project.Spec.Namespace != v1beta1constants.GardenNamespace && !strings.HasPrefix(*project.Spec.Namespace, gardenerutils.ProjectNamespacePrefix) {
		return admission.NewForbidden(a, fmt.Errorf(".spec.namespace must start with %s", gardenerutils.ProjectNamespacePrefix))
	}

	return nil
}
