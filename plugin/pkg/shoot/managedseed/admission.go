// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseed

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/core"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootManagedSeed"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ManagedSeed contains listers and and admission handler.
type ManagedSeed struct {
	*admission.Handler
	coreClient           coreclientset.Interface
	seedManagementClient seedmanagementclientset.Interface
	readyFunc            admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreClientset(&ManagedSeed{})
	_ = admissioninitializer.WantsSeedManagementClientset(&ManagedSeed{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new ManagedSeed admission plugin.
func New() (*ManagedSeed, error) {
	return &ManagedSeed{
		Handler: admission.NewHandler(admission.Delete),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ManagedSeed) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetInternalCoreClientset sets the garden core clientset.
func (v *ManagedSeed) SetInternalCoreClientset(c coreclientset.Interface) {
	v.coreClient = c
}

// SetSeedManagementClientset sets the garden seedmanagement clientset.
func (v *ManagedSeed) SetSeedManagementClientset(c seedmanagementclientset.Interface) {
	v.seedManagementClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ManagedSeed) ValidateInitialization() error {
	if v.coreClient == nil {
		return errors.New("missing garden core client")
	}
	if v.seedManagementClient == nil {
		return errors.New("missing garden seedmanagement client")
	}
	return nil
}

var _ admission.ValidationInterface = &ManagedSeed{}

// Validate validates if the Shoot can be deleted. If the
func (v *ManagedSeed) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if v.readyFunc == nil {
		v.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !v.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	switch {
	case a.GetName() == "":
		return v.validateDeleteCollection(ctx, a)
	default:
		return v.validateDelete(ctx, a)
	}
}

func (v *ManagedSeed) validateDeleteCollection(ctx context.Context, a admission.Attributes) error {
	shoots, err := v.getShoots(ctx, labels.Everything())
	if err != nil {
		return err
	}
	for _, shoot := range shoots {
		if err := v.validateDelete(ctx, newAttributesWithName(a, shoot.Name)); err != nil {
			return err
		}
	}

	return nil
}

func (v *ManagedSeed) validateDelete(ctx context.Context, a admission.Attributes) error {
	managedSeed, err := kutil.GetManagedSeed(ctx, v.seedManagementClient, a.GetNamespace(), a.GetName())
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not get ManagedSeed for shoot '%s/%s': %v", a.GetNamespace(), a.GetName(), err))
	}
	if managedSeed == nil {
		return nil
	}

	return admission.NewForbidden(a, fmt.Errorf("cannot delete shoot %s/%s since it is still referenced by a managed seed", a.GetNamespace(), a.GetName()))
}

func newAttributesWithName(a admission.Attributes, name string) admission.Attributes {
	return admission.NewAttributesRecord(a.GetObject(),
		a.GetOldObject(),
		a.GetKind(),
		a.GetNamespace(),
		name,
		a.GetResource(),
		a.GetSubresource(),
		a.GetOperation(),
		a.GetOperationOptions(),
		a.IsDryRun(),
		a.GetUserInfo())
}

func (v *ManagedSeed) getShoots(ctx context.Context, selector labels.Selector) ([]core.Shoot, error) {
	shootList, err := v.coreClient.Core().Shoots("").List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return shootList.Items, nil
}
