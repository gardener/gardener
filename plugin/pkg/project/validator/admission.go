// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	plugin "github.com/gardener/gardener/plugin/pkg"
	"github.com/gardener/gardener/plugin/pkg/utils"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameProjectValidator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

type handler struct {
	*admission.Handler
}

// New creates a new handler admission plugin.
func New() (*handler, error) {
	return &handler{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

var _ admission.MutationInterface = &handler{}

func (v *handler) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	// TODO: Remove this check in favor of static validation in a future release, see https://github.com/gardener/gardener/pull/4228.
	if project.Spec.Namespace != nil && *project.Spec.Namespace != v1beta1constants.GardenNamespace && !strings.HasPrefix(*project.Spec.Namespace, gardenerutils.ProjectNamespacePrefix) {
		return admission.NewForbidden(a, fmt.Errorf(".spec.namespace must start with %s", gardenerutils.ProjectNamespacePrefix))
	}

	if utils.SkipVerification(a.GetOperation(), project.ObjectMeta) {
		return nil
	}

	if a.GetOperation() == admission.Create {
		ensureProjectOwner(project, a.GetUserInfo().GetName())
	}

	ensureOwnerIsMember(project)

	return nil
}

func ensureProjectOwner(project *gardencore.Project, userName string) {
	// Set createdBy field in Project
	project.Spec.CreatedBy = &rbacv1.Subject{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     rbacv1.UserKind,
		Name:     userName,
	}

	if project.Spec.Owner == nil {
		project.Spec.Owner = func() *rbacv1.Subject {
			for _, member := range project.Spec.Members {
				for _, role := range member.Roles {
					if role == gardencore.ProjectMemberOwner {
						return member.Subject.DeepCopy()
					}
				}
			}
			return project.Spec.CreatedBy
		}()
	}
}

func ensureOwnerIsMember(project *gardencore.Project) {
	if project.Spec.Owner == nil {
		return
	}

	ownerIsMember := slices.ContainsFunc(project.Spec.Members, func(member gardencore.ProjectMember) bool {
		return member.Subject == *project.Spec.Owner
	})

	if !ownerIsMember {
		project.Spec.Members = append(project.Spec.Members, gardencore.ProjectMember{
			Subject: *project.Spec.Owner,
			Roles: []string{
				gardencore.ProjectMemberAdmin,
				gardencore.ProjectMemberOwner,
			},
		})
	}
}
