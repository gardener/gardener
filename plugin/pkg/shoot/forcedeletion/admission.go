// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package forcedeletion

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

var (
	errorCodesAllowingForceDeletion = sets.New(
		core.ErrorInfraUnauthenticated,
		core.ErrorInfraUnauthorized,
		core.ErrorInfraDependencies,
		core.ErrorCleanupClusterResources,
		core.ErrorConfigurationProblem,
	)
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootForceDeletion, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ForceDeletion contains the admission handler.
type ForceDeletion struct {
	*admission.Handler
}

// New creates a new ForceDeletion admission plugin.
func New() (*ForceDeletion, error) {
	return &ForceDeletion{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

var _ admission.ValidationInterface = &ForceDeletion{}

// Validate validates the force delete annotation of a Shoot.
func (v *ForceDeletion) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	shoot, convertIsSuccessful := a.GetObject().(*core.Shoot)
	if !convertIsSuccessful {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	if a.GetOperation() == admission.Create {
		if err := validateForceDeletion(shoot, nil); err != nil {
			return admission.NewForbidden(a, fmt.Errorf("%+v", err))
		}
	} else if a.GetOperation() == admission.Update {
		oldShoot, convertIsSuccessful := a.GetOldObject().(*core.Shoot)

		if !convertIsSuccessful {
			return apierrors.NewInternalError(errors.New("could not convert old resource into Shoot object"))
		}

		if err := validateForceDeletion(shoot, oldShoot); err != nil {
			return admission.NewForbidden(a, fmt.Errorf("%+v", err))
		}
	}

	return nil
}

func validateForceDeletion(newShoot, oldShoot *core.Shoot) *field.Error {
	var (
		fldPath               = field.NewPath("metadata", "annotations").Key(v1beta1constants.AnnotationConfirmationForceDeletion)
		oldNeedsForceDeletion = helper.ShootNeedsForceDeletion(oldShoot)
		newNeedsForceDeletion = helper.ShootNeedsForceDeletion(newShoot)
	)

	if !newNeedsForceDeletion && oldNeedsForceDeletion {
		return field.Forbidden(fldPath, "force-deletion annotation cannot be removed once set")
	}

	if newNeedsForceDeletion && !oldNeedsForceDeletion {
		if newShoot.DeletionTimestamp == nil {
			return field.Forbidden(fldPath, "force-deletion annotation cannot be set when Shoot deletionTimestamp is nil")
		}

		for _, lastError := range newShoot.Status.LastErrors {
			if errorCodesAllowingForceDeletion.HasAny(lastError.Codes...) {
				return nil
			}
		}

		return field.Forbidden(fldPath, fmt.Sprintf("force-deletion annotation cannot be set when Shoot status does not contain one of these ErrorCode: %v", sets.List(errorCodesAllowingForceDeletion)))
	}

	return nil
}
