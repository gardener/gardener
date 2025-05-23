// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	plugin "github.com/gardener/gardener/plugin/pkg"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameManagedSeedShoot, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// Shoot contains listers and admission handler.
type Shoot struct {
	*admission.Handler
	shootLister          gardencorev1beta1listers.ShootLister
	seedManagementClient seedmanagementclientset.Interface
	readyFunc            admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&Shoot{})
	_ = admissioninitializer.WantsSeedManagementClientSet(&Shoot{})

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

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *Shoot) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	shootInformer := f.Core().V1beta1().Shoots()
	v.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced)
}

// SetSeedManagementClientSet sets the garden seedmanagement clientset.
func (v *Shoot) SetSeedManagementClientSet(c seedmanagementclientset.Interface) {
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

func (v *Shoot) getShoots(selector labels.Selector) ([]*gardencorev1beta1.Shoot, error) {
	shoots, err := v.shootLister.List(selector)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	return shoots, nil
}
