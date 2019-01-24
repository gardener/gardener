// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package resources

import (
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/core"

	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ControllerRegistrationResources"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(config io.Reader) (admission.Interface, error) {
	return New()
}

// Resources contains an admission handler and listers.
type Resources struct {
	*admission.Handler
	coreClient coreclientset.Interface
	readyFunc  admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreClientset(&Resources{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new Resources admission plugin.
func New() (*Resources, error) {
	return &Resources{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (r *Resources) AssignReadyFunc(f admission.ReadyFunc) {
	r.readyFunc = f
	r.SetReadyFunc(f)
}

// SetInternalCoreClientset gets the clientset from the Kubernetes client.
func (r *Resources) SetInternalCoreClientset(c coreclientset.Interface) {
	r.coreClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (r *Resources) ValidateInitialization() error {
	return nil
}

// Validate makes admissions decisions based on the resources specified in a ControllerRegistration object.
// It does reject the request if there is any other existing ControllerRegistration object in the system that
// specifies the same resource kind/type combination like the incoming object.
func (r *Resources) Validate(a admission.Attributes) error {
	// Wait until the caches have been synced
	if r.readyFunc == nil {
		r.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !r.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot or Project.
	// TODO: in future the Kinds should be configurable
	// https://v1-9.docs.kubernetes.io/docs/admin/admission-controllers/#imagepolicywebhook
	if a.GetKind().GroupKind() != core.Kind("ControllerRegistration") {
		return nil
	}
	controllerRegistration, ok := a.GetObject().(*core.ControllerRegistration)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into ControllerRegistration object")
	}

	// Live lookup to prevent missing any data
	controllerRegistrationList, err := r.coreClient.Core().ControllerRegistrations().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	existingResources := map[string]string{}
	for _, obj := range controllerRegistrationList.Items {
		for _, resource := range obj.Spec.Resources {
			existingResources[resource.Kind] = resource.Type
		}
	}

	for _, resource := range controllerRegistration.Spec.Resources {
		if t, ok := existingResources[resource.Kind]; ok && t == resource.Type {
			return admission.NewForbidden(a, fmt.Errorf("another ControllerRegistration resource already exists that supports resource %s/%s", resource.Kind, resource.Type))
		}
	}

	return nil
}
