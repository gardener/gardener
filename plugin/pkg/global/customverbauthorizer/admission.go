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

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "CustomVerbAuthorizer"

	// CustomVerbModifyProjectTolerationsWhitelist is a constant for the custom verb that allows modifying the
	// `.spec.tolerations.whitelist` field in `Project` resources.
	CustomVerbModifyProjectTolerationsWhitelist = "modify-spec-tolerations-whitelist"
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

	if apiequality.Semantic.DeepEqual(oldObj.Spec.Tolerations, obj.Spec.Tolerations) {
		return nil
	}

	return c.authorize(ctx, a, CustomVerbModifyProjectTolerationsWhitelist, "modify .spec.tolerations.whitelist")
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
