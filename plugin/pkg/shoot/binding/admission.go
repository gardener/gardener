// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, b. 2 except as noted otherwise in the LICENSE file
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

package binding

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/features"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	externalcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	corev1alpha1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootBinding"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// Binding contains listers and and admission handler.
type Binding struct {
	*admission.Handler
	seedLister       corelisters.SeedLister
	shootLister      corelisters.ShootLister
	shootStateLister corev1alpha1listers.ShootStateLister
	readyFunc        admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&Binding{})
	_ = admissioninitializer.WantsExternalCoreInformerFactory(&Binding{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new Binding admission plugin.
func New() (*Binding, error) {
	return &Binding{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (b *Binding) AssignReadyFunc(f admission.ReadyFunc) {
	b.readyFunc = f
	b.SetReadyFunc(f)
}

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (b *Binding) SetInternalCoreInformerFactory(f coreinformers.SharedInformerFactory) {
	seedInformer := f.Core().InternalVersion().Seeds()
	b.seedLister = seedInformer.Lister()

	shootInformer := f.Core().InternalVersion().Shoots()
	b.shootLister = shootInformer.Lister()

	readyFuncs = append(
		readyFuncs,
		seedInformer.Informer().HasSynced,
		shootInformer.Informer().HasSynced,
	)
}

// SetExternalCoreInformerFactory sets the external garden core informer factory.
func (b *Binding) SetExternalCoreInformerFactory(f externalcoreinformers.SharedInformerFactory) {
	shootStateInformer := f.Core().V1alpha1().ShootStates()
	b.shootStateLister = shootStateInformer.Lister()

	readyFuncs = append(readyFuncs, shootStateInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (b *Binding) ValidateInitialization() error {
	if b.seedLister == nil {
		return errors.New("missing seed lister")
	}
	if b.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if b.shootStateLister == nil {
		return errors.New("missing shoot state lister")
	}
	return nil
}

var _ admission.ValidationInterface = &Binding{}

// Validate validates the Binding against the existing conditions.
func (b *Binding) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if b.readyFunc == nil {
		b.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !b.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Binding
	if a.GetKind().GroupKind() != core.Kind("Binding") {
		return nil
	}

	binding, convertIsSuccessful := a.GetObject().(*core.Binding)
	if !convertIsSuccessful {
		return apierrors.NewInternalError(errors.New("could not convert resource into Binding object"))
	}

	if len(binding.Target.Kind) == 0 || binding.Target.Kind != "Seed" {
		return field.NotSupported(field.NewPath("target", "kind"), binding.Target.Kind, []string{"Seed"})
	}

	if len(binding.Target.Name) == 0 {
		return field.Required(field.NewPath("target", "name"), "target name for Binding object cannot be an empty string")
	}

	shoot, err := b.shootLister.Shoots(binding.Namespace).Get(binding.Name)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not find corresponding shoot %q: %+v", binding.Name, err.Error()))
	}

	if shoot.Spec.SeedName != nil && !utilfeature.DefaultFeatureGate.Enabled(features.SeedChange) {
		return apivalidation.ValidateImmutableField(binding.Target.Name, shoot.Spec.SeedName, field.NewPath("target", "name")).ToAggregate()
	}

	seed, err := b.seedLister.Get(binding.Target.Name)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not find referenced seed %q: %+v", binding.Target.Name, err.Error()))
	}

	validationContext := &validationContext{
		seed:    seed,
		shoot:   shoot,
		binding: binding,
	}

	return validationContext.validateScheduling(a, b.shootLister, b.seedLister, b.shootStateLister)
}

type validationContext struct {
	seed    *core.Seed
	shoot   *core.Shoot
	binding *core.Binding
}

func (c *validationContext) validateScheduling(a admission.Attributes, shootLister corelisters.ShootLister, seedLister corelisters.SeedLister, shootStateLister corev1alpha1listers.ShootStateLister) error {
	var (
		// Ideally, the initial scheduling is always done by the scheduler. In that case, all these checks are already
		// performed by the scheduler before assigning the seed. But if a operator tries to create the binding for a shoot
		// which doesn't have a seed assigned by the scheduler yet, we still need these checks.
		shootIsBeingScheduled          = c.shoot.Spec.SeedName == nil
		shootIsBeingRescheduled        = c.shoot.Spec.SeedName != nil && *c.shoot.Spec.SeedName != c.binding.Target.Name
		mustCheckSchedulingConstraints = shootIsBeingScheduled || shootIsBeingRescheduled
	)

	if mustCheckSchedulingConstraints {
		if c.seed.DeletionTimestamp != nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", c.shoot.Name, c.seed.Name))
		}

		if !helper.TaintsAreTolerated(c.seed.Spec.Taints, c.shoot.Spec.Tolerations) {
			return admission.NewForbidden(a, fmt.Errorf("forbidden to use a seeds whose taints are not tolerated by the shoot"))
		}

		if allocatableShoots, ok := c.seed.Status.Allocatable[core.ResourceShoots]; ok {
			scheduledShoots, err := getNumberOfShootsOnSeed(shootLister, c.seed.Name)
			if err != nil {
				return apierrors.NewInternalError(err)
			}

			if scheduledShoots >= allocatableShoots.Value() {
				return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' that already has the maximum number of shoots scheduled on it (%d)", c.shoot.Name, c.seed.Name, allocatableShoots.Value()))
			}
		}
	}

	if shootIsBeingRescheduled {
		oldSeed, err := seedLister.Get(*c.shoot.Spec.SeedName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find referenced seed: %+v", err.Error()))
		}

		if oldSeed.Spec.Backup == nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot change seed name because backup is not configured for old seed %q", oldSeed.Name))
		}
		if c.seed.Spec.Backup == nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot change seed name because backup is not configured for seed %q", c.seed.Name))
		}

		if oldSeed.Spec.Provider.Type != c.seed.Spec.Provider.Type {
			return admission.NewForbidden(a, fmt.Errorf("cannot change seed because cloud provider for new seed (%s) is not equal to cloud provider for old seed (%s)", c.seed.Spec.Provider.Type, oldSeed.Spec.Provider.Type))
		}

		// Check if ShootState contains the new etcd-encryption key after it got migrated to the new secrets manager
		// with https://github.com/gardener/gardener/pull/5616
		shootState, err := shootStateLister.ShootStates(c.shoot.Namespace).Get(c.shoot.Name)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find shoot state: %+v", err.Error()))
		}

		etcdEncryptionFound := false

		for _, data := range shootState.Spec.Gardener {
			if data.Labels[secretsmanager.LabelKeyName] == "kube-apiserver-etcd-encryption-key" &&
				data.Labels[secretsmanager.LabelKeyManagedBy] == secretsmanager.LabelValueSecretsManager {
				etcdEncryptionFound = true
				break
			}
		}

		if !etcdEncryptionFound {
			return admission.NewForbidden(a, errors.New("cannot change seed because etcd encryption key not found in shoot state - please reconcile the shoot first"))
		}
	}

	return nil
}

func getNumberOfShootsOnSeed(shootLister corelisters.ShootLister, seedName string) (int64, error) {
	allShoots, err := shootLister.Shoots(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return 0, fmt.Errorf("could not list all shoots: %w", err)
	}

	seedUsage := helper.CalculateSeedUsage(allShoots)
	return int64(seedUsage[seedName]), nil
}
