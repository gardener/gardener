// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package customverbauthorizer

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"

	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "CustomVerbAuthorizer"

	// CustomVerbModifyProjectTolerationsWhitelist is a constant for the custom verb that allows modifying the
	// `.spec.tolerations.whitelist` field in `Project` resources.
	CustomVerbModifyProjectTolerationsWhitelist = "modify-spec-tolerations-whitelist"
	// CustomVerbProjectUserAccessManagement is a constant for the custom verb that allows to manage human users or
	// groups subjects in the `.spec.members` field in `Project` resources.
	CustomVerbProjectManageMembers = "manage-members"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(config io.Reader) (admission.Interface, error) {
	return New()
}

// CustomVerbAuthorizer contains an admission handler and listers.
type CustomVerbAuthorizer struct {
	*admission.Handler
	authorizer authorizer.Authorizer
}

var _ = admissioninitializer.WantsAuthorizer(&CustomVerbAuthorizer{})

// New creates a new CustomVerbAuthorizer admission plugin.
func New() (*CustomVerbAuthorizer, error) {
	return &CustomVerbAuthorizer{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// SetAuthorizer gets the authorizer.
func (c *CustomVerbAuthorizer) SetAuthorizer(authorizer authorizer.Authorizer) {
	c.authorizer = authorizer
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (c *CustomVerbAuthorizer) ValidateInitialization() error {
	return nil
}

var _ admission.ValidationInterface = &CustomVerbAuthorizer{}

// Validate makes admissions decisions based on custom verbs.
func (c *CustomVerbAuthorizer) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	switch a.GetKind().GroupKind() {
	case core.Kind("Project"):
		return c.admitProjects(ctx, a)
	}

	return nil
}

func (c *CustomVerbAuthorizer) admitProjects(ctx context.Context, a admission.Attributes) error {
	var (
		oldObj = &core.Project{}
		obj    = &core.Project{}
		ok     bool
	)

	obj, ok = a.GetObject().(*core.Project)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Project object")
	}

	if a.GetOperation() == admission.Update {
		oldObj, ok = a.GetOldObject().(*core.Project)
		if !ok {
			return apierrors.NewBadRequest("could not convert old resource into Project object")
		}
	}

	if mustCheckProjectTolerationsWhitelist(oldObj.Spec.Tolerations, obj.Spec.Tolerations) {
		return c.authorize(ctx, a, CustomVerbModifyProjectTolerationsWhitelist, "modify .spec.tolerations.whitelist")
	}

	if mustCheckProjectMembers(oldObj.Spec.Members, obj.Spec.Members) {
		return c.authorize(ctx, a, CustomVerbProjectManageMembers, "manage human users or groups in .spec.members")
	}

	return nil
}

func (c *CustomVerbAuthorizer) authorize(ctx context.Context, a admission.Attributes, verb, operation string) error {
	var (
		userInfo  = a.GetUserInfo()
		resource  = a.GetResource()
		namespace = a.GetNamespace()
		name      = a.GetName()
	)

	decision, _, err := c.authorizer.Authorize(ctx, authorizer.AttributesRecord{
		User:            userInfo,
		APIGroup:        resource.Group,
		Resource:        resource.Resource,
		Namespace:       namespace,
		Name:            name,
		Verb:            verb,
		ResourceRequest: true,
	})
	if err != nil {
		return err
	}
	if decision != authorizer.DecisionAllow {
		return admission.NewForbidden(a, fmt.Errorf("user %q is not allowed to %s for %q", userInfo.GetName(), operation, resource.Resource))
	}

	return nil
}

func mustCheckProjectTolerationsWhitelist(oldTolerations, tolerations *core.ProjectTolerations) bool {
	if apiequality.Semantic.DeepEqual(oldTolerations, tolerations) {
		return false
	}
	if oldTolerations == nil && tolerations != nil {
		return !apiequality.Semantic.DeepEqual(nil, tolerations.Whitelist)
	}
	if oldTolerations != nil && tolerations == nil {
		return !apiequality.Semantic.DeepEqual(oldTolerations.Whitelist, nil)
	}
	return !apiequality.Semantic.DeepEqual(oldTolerations.Whitelist, tolerations.Whitelist)
}

func mustCheckProjectMembers(oldMembers, members []core.ProjectMember) bool {
	if apiequality.Semantic.DeepEqual(oldMembers, members) {
		return false
	}

	var oldHumanUsers, newHumanUsers = findHumanUsers(oldMembers), findHumanUsers(members)
	return !oldHumanUsers.Equal(newHumanUsers)
}

func findHumanUsers(members []core.ProjectMember) sets.String {
	result := sets.NewString()

	for _, member := range members {
		if isHumanUser(member.Subject) {
			result.Insert(member.Kind + member.Name)
		}
	}

	return result
}

func isHumanUser(subject rbacv1.Subject) bool {
	return subject.Kind == rbacv1.UserKind && !strings.HasPrefix(subject.Name, serviceaccount.ServiceAccountUsernamePrefix)
}
