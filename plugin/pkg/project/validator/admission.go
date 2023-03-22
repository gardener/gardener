// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ProjectValidator"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

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

	// TODO: Remove this admission plugin in favor of static validation in a future release, see https://github.com/gardener/gardener/pull/4228.
	if project.Spec.Namespace != nil && *project.Spec.Namespace != v1beta1constants.GardenNamespace && !strings.HasPrefix(*project.Spec.Namespace, gardenerutils.ProjectNamespacePrefix) {
		return admission.NewForbidden(a, fmt.Errorf(".spec.namespace must start with %s", gardenerutils.ProjectNamespacePrefix))
	}

	return nil
}
