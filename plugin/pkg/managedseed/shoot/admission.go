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

package shoot

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ManagedSeedShoot"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// Shoot contains listers and admission handler.
type Shoot struct {
	*admission.Handler
	shootLister          gardencorelisters.ShootLister
	seedManagementClient seedmanagementclientset.Interface
	readyFunc            admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&Shoot{})
	_ = admissioninitializer.WantsSeedManagementClientset(&Shoot{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new Shoot admission plugin.
func New() (*Shoot, error) {
	return &Shoot{
		Handler: admission.NewHandler(admission.Delete),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *Shoot) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *Shoot) SetInternalCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	shootInformer := f.Core().InternalVersion().Shoots()
	v.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced)
}

// SetSeedManagementClientset sets the garden seedmanagement clientset.
func (v *Shoot) SetSeedManagementClientset(c seedmanagementclientset.Interface) {
	v.seedManagementClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *Shoot) ValidateInitialization() error {
	if v.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if v.seedManagementClient == nil {
		return errors.New("missing garden seedmanagement client")
	}
	return nil
}

var _ admission.ValidationInterface = &Shoot{}

// Validate validates if the ManagedSeed can be deleted.
func (v *Shoot) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	// Ignore all kinds other than ManagedSeed
	if a.GetKind().GroupKind() != seedmanagementv1alpha1.Kind("ManagedSeed") {
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

func (v *Shoot) validateDeleteCollection(ctx context.Context, a admission.Attributes) error {
	managedSeeds, err := v.getManagedSeeds(ctx, labels.Everything())
	if err != nil {
		return err
	}
	for _, managedSeed := range managedSeeds {
		if err := v.validateDelete(ctx, newAttributesWithName(a, managedSeed.Name)); err != nil {
			return err
		}
	}

	return nil
}

func (v *Shoot) validateDelete(_ context.Context, a admission.Attributes) error {
	seedName := a.GetName()

	shoots, err := v.getShoots(labels.Everything())
	if err != nil {
		return err
	}

	if admissionutils.IsSeedUsedByShoot(seedName, shoots) {
		return admission.NewForbidden(a, fmt.Errorf("cannot delete managed seed %s/%s since its seed %s is still used by shoot(s)", a.GetNamespace(), a.GetName(), a.GetName()))
	}

	return nil
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

func (v *Shoot) getManagedSeeds(ctx context.Context, selector labels.Selector) ([]seedmanagementv1alpha1.ManagedSeed, error) {
	managedSeedList, err := v.seedManagementClient.SeedmanagementV1alpha1().ManagedSeeds("").List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return managedSeedList.Items, nil
}

func (v *Shoot) getShoots(selector labels.Selector) ([]*core.Shoot, error) {
	shoots, err := v.shootLister.List(selector)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	return shoots, nil
}
