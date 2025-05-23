// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameControllerRegistrationResources, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(_ io.Reader) (admission.Interface, error) {
	return New()
}

// Resources contains an admission handler and listers.
type Resources struct {
	*admission.Handler
	coreClient gardencoreclientset.Interface
	readyFunc  admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreClientSet(&Resources{})

	readyFuncs []admission.ReadyFunc
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

// SetCoreClientSet gets the clientset from the Kubernetes client.
func (r *Resources) SetCoreClientSet(c gardencoreclientset.Interface) {
	r.coreClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (r *Resources) ValidateInitialization() error {
	return nil
}

var _ admission.ValidationInterface = &Resources{}

// Validate makes admissions decisions based on the resources specified in a ControllerRegistration object.
// It does reject the request if there is any other existing ControllerRegistration object in the system that
// specifies the same resource kind/type combination like the incoming object.
func (r *Resources) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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
	if a.GetKind().GroupKind() != core.Kind("ControllerRegistration") {
		return nil
	}
	controllerRegistration, ok := a.GetObject().(*core.ControllerRegistration)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into ControllerRegistration object")
	}

	// Live lookup to prevent missing any data
	controllerRegistrationList, err := r.coreClient.CoreV1beta1().ControllerRegistrations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	existingResources := map[string]string{}
	for _, obj := range controllerRegistrationList.Items {
		if obj.Name == controllerRegistration.Name {
			continue
		}

		for _, resource := range obj.Spec.Resources {
			if resource.Primary != nil && !*resource.Primary {
				continue
			}

			existingResources[resource.Kind] = resource.Type
		}
	}

	for _, resource := range controllerRegistration.Spec.Resources {
		if resource.Primary != nil && !*resource.Primary {
			continue
		}

		if t, ok := existingResources[resource.Kind]; ok && t == resource.Type {
			return admission.NewForbidden(a, fmt.Errorf("another ControllerRegistration resource already exists that controls resource %s/%s primarily", resource.Kind, resource.Type))
		}
	}

	return nil
}
