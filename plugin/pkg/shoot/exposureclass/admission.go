// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package exposureclass

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootExposureClass, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ExposureClass contains listers and admission handler.
type ExposureClass struct {
	*admission.Handler

	exposureClassLister gardencorev1beta1listers.ExposureClassLister
	readyFunc           admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&ExposureClass{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new ExposureClass admission plugin.
func New() (*ExposureClass, error) {
	return &ExposureClass{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (e *ExposureClass) AssignReadyFunc(f admission.ReadyFunc) {
	e.readyFunc = f
	e.SetReadyFunc(f)
}

// SetCoreInformerFactory sets the external garden core informer factory.
func (e *ExposureClass) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	exposureClassInformer := f.Core().V1beta1().ExposureClasses()
	e.exposureClassLister = exposureClassInformer.Lister()

	readyFuncs = append(readyFuncs, exposureClassInformer.Informer().HasSynced)
}

func (e *ExposureClass) waitUntilReady(attrs admission.Attributes) error {
	// Wait until the caches have been synced
	if e.readyFunc == nil {
		e.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}

	if !e.WaitForReady() {
		return admission.NewForbidden(attrs, errors.New("not yet ready to handle request"))
	}

	return nil
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (e *ExposureClass) ValidateInitialization() error {
	if e.exposureClassLister == nil {
		return errors.New("missing ExposureClass lister")
	}
	return nil
}

// Admit unite the seed selector and/or tolerations of a Shoot resource
// with the ones from the referenced ExposureClass.
func (e *ExposureClass) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	if err := e.waitUntilReady(a); err != nil {
		return fmt.Errorf("err while waiting for ready %w", err)
	}

	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	// Ignore any updates to shoot subresources.
	if a.GetSubresource() != "" {
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	if err := e.admitShoot(shoot); err != nil {
		return admission.NewForbidden(a, err)
	}

	return nil
}

func (e *ExposureClass) admitShoot(shoot *core.Shoot) error {
	if shoot.Spec.ExposureClassName == nil {
		return nil
	}

	exposureClass, err := e.exposureClassLister.Get(*shoot.Spec.ExposureClassName)
	if err != nil {
		return err
	}

	if exposureClass.Scheduling == nil {
		return nil
	}

	targetSeedSelector, err := uniteSeedSelectors(shoot.Spec.SeedSelector, exposureClass.Scheduling.SeedSelector)
	if err != nil {
		return err
	}
	shoot.Spec.SeedSelector = targetSeedSelector

	targetTolerations, err := uniteTolerations(shoot.Spec.Tolerations, exposureClass.Scheduling.Tolerations)
	if err != nil {
		return err
	}
	shoot.Spec.Tolerations = targetTolerations

	return nil
}

func uniteSeedSelectors(shootSeedSelector *core.SeedSelector, exposureClassSeedSelector *gardencorev1beta1.SeedSelector) (*core.SeedSelector, error) {
	if exposureClassSeedSelector == nil {
		return shootSeedSelector, nil
	}

	if shootSeedSelector == nil {
		shootSeedSelector = &core.SeedSelector{}
	}

	// Unite matching labels.
	if labels.Conflicts(shootSeedSelector.MatchLabels, exposureClassSeedSelector.MatchLabels) {
		return nil, fmt.Errorf("matching labels of the seed selector conflicts with the ones of referenced exposureclass")
	}
	shootSeedSelector.MatchLabels = labels.Merge(shootSeedSelector.MatchLabels, exposureClassSeedSelector.MatchLabels)

	// Unite matching expressions.
	shootSeedSelector.MatchExpressions = append(shootSeedSelector.MatchExpressions, exposureClassSeedSelector.MatchExpressions...)

	// Unite provider types.
	shootProviderTypes := sets.New[string]().Insert(shootSeedSelector.ProviderTypes...)
	exposureclasssProviderTypes := sets.New[string]().Insert(exposureClassSeedSelector.ProviderTypes...)
	shootSeedSelector.ProviderTypes = sets.List(shootProviderTypes.Union(exposureclasssProviderTypes))

	return shootSeedSelector, nil
}

func uniteTolerations(shootTolerations []core.Toleration, exposureClassTolerations []gardencorev1beta1.Toleration) ([]core.Toleration, error) {
	shootTolerationsKeys := sets.New[string]()
	for _, toleration := range shootTolerations {
		shootTolerationsKeys.Insert(toleration.Key)
	}

	for _, toleration := range exposureClassTolerations {
		if shootTolerationsKeys.Has(toleration.Key) {
			return nil, fmt.Errorf("toleration with key %q conflicts with the ones of referenced exposureclass", toleration.Key)
		}

		shootTolerations = append(shootTolerations, core.Toleration{Key: toleration.Key, Value: toleration.Value})
	}

	return shootTolerations, nil
}
