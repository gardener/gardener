// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	plugin "github.com/gardener/gardener/plugin/pkg"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameSeedValidator, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ValidateSeed contains listers and admission handler.
type ValidateSeed struct {
	*admission.Handler
	seedLister  gardencorelisters.SeedLister
	shootLister gardencorelisters.ShootLister
	readyFunc   admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&ValidateSeed{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new ValidateSeed admission plugin.
func New() (*ValidateSeed, error) {
	return &ValidateSeed{
		Handler: admission.NewHandler(admission.Delete, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ValidateSeed) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateSeed) SetInternalCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	seedInformer := f.Core().InternalVersion().Seeds()
	v.seedLister = seedInformer.Lister()

	shootInformer := f.Core().InternalVersion().Shoots()
	v.shootLister = shootInformer.Lister()

	backupBucketInformer := f.Core().InternalVersion().BackupBuckets()

	readyFuncs = append(readyFuncs, seedInformer.Informer().HasSynced, shootInformer.Informer().HasSynced, backupBucketInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ValidateSeed) ValidateInitialization() error {
	if v.seedLister == nil {
		return errors.New("missing seed lister")
	}
	if v.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	return nil
}

var _ admission.ValidationInterface = &ValidateSeed{}

// Validate validates the Seed details against existing Shoots and BackupBuckets
func (v *ValidateSeed) Validate(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	// Ignore all kinds other than Seed
	if a.GetKind().GroupKind() != core.Kind("Seed") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	switch a.GetOperation() {
	case admission.Update:
		return v.validateSeedUpdate(a)
	case admission.Delete:
		return v.validateSeedDeletion(a)
	}

	return nil
}

func (v *ValidateSeed) validateSeedUpdate(a admission.Attributes) error {
	oldSeed, newSeed, err := getOldAndNewSeeds(a)
	if err != nil {
		return err
	}

	return admissionutils.ValidateZoneRemovalFromSeeds(&oldSeed.Spec, &newSeed.Spec, newSeed.Name, v.shootLister, "Seed")
}

func (v *ValidateSeed) validateSeedDeletion(a admission.Attributes) error {
	seedName := a.GetName()

	shoots, err := v.shootLister.List(labels.Everything())
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	if admissionutils.IsSeedUsedByShoot(seedName, shoots) {
		return admission.NewForbidden(a, fmt.Errorf("cannot delete seed %s since it is still used by shoot(s)", seedName))
	}
	return nil
}

func getOldAndNewSeeds(attrs admission.Attributes) (*core.Seed, *core.Seed, error) {
	var (
		oldSeed, newSeed *core.Seed
		ok               bool
	)

	if oldSeed, ok = attrs.GetOldObject().(*core.Seed); !ok {
		return nil, nil, apierrors.NewInternalError(errors.New("failed to convert old resource into Seed object"))
	}

	if newSeed, ok = attrs.GetObject().(*core.Seed); !ok {
		return nil, nil, apierrors.NewInternalError(errors.New("failed to convert new resource into Seed object"))
	}

	return oldSeed, newSeed, nil
}
