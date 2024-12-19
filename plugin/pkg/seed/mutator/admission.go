// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	seedmanagementinformers "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions"
	seedmanagementv1alpha1listers "github.com/gardener/gardener/pkg/client/seedmanagement/listers/seedmanagement/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameSeedMutator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// MutateSeed contains listers and admission handler.
type MutateSeed struct {
	*admission.Handler
	managedSeedLister seedmanagementv1alpha1listers.ManagedSeedLister
	shootLister       gardencorev1beta1listers.ShootLister
	readyFunc         admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&MutateSeed{})
	_ = admissioninitializer.WantsSeedManagementInformerFactory(&MutateSeed{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new MutateSeed admission plugin.
func New() (*MutateSeed, error) {
	return &MutateSeed{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (m *MutateSeed) AssignReadyFunc(f admission.ReadyFunc) {
	m.readyFunc = f
	m.SetReadyFunc(f)
}

// SetSeedManagementInformerFactory gets Lister from SharedInformerFactory.
func (m *MutateSeed) SetSeedManagementInformerFactory(f seedmanagementinformers.SharedInformerFactory) {
	managedSeedInformer := f.Seedmanagement().V1alpha1().ManagedSeeds()
	m.managedSeedLister = managedSeedInformer.Lister()

	readyFuncs = append(readyFuncs, managedSeedInformer.Informer().HasSynced)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (m *MutateSeed) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	shootInformer := f.Core().V1beta1().Shoots()
	m.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (m *MutateSeed) ValidateInitialization() error {
	if m.managedSeedLister == nil {
		return errors.New("missing managed seed lister")
	}
	if m.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	return nil
}

var _ admission.MutationInterface = &MutateSeed{}

// Admit mutates the Seed.
func (m *MutateSeed) Admit(_ context.Context, attrs admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if m.readyFunc == nil {
		m.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}

	if !m.WaitForReady() {
		return admission.NewForbidden(attrs, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Seed
	if attrs.GetKind().GroupKind() != core.Kind("Seed") {
		return nil
	}

	seed, ok := attrs.GetObject().(*core.Seed)
	if !ok {
		return apierrors.NewInternalError(errors.New("failed to convert new resource into Seed object"))
	}

	seedNames := []*string{&seed.Name}

	managedSeed, err := m.managedSeedLister.ManagedSeeds(v1beta1constants.GardenNamespace).Get(seed.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get managed seed %q: %v", seed.Name, err)
		}
	} else if managedSeed.Spec.Shoot != nil {
		shoot, err := m.shootLister.Shoots(managedSeed.Namespace).Get(managedSeed.Spec.Shoot.Name)
		if err != nil {
			return fmt.Errorf("failed to get shoot %s for managed seed %q: %v", managedSeed.Spec.Shoot.Name, managedSeed.Name, err)
		}
		seedNames = append(seedNames, shoot.Spec.SeedName)
	}

	gardenerutils.MaintainSeedNameLabels(seed, seedNames...)
	return nil
}
