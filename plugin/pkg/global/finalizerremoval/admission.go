// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizerremoval

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/security"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameFinalizerRemoval, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// FinalizerRemoval contains listers and admission handler.
type FinalizerRemoval struct {
	*admission.Handler
	shootLister gardencorev1beta1listers.ShootLister
	readyFunc   admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&FinalizerRemoval{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new FinalizerRemoval admission plugin.
func New() (*FinalizerRemoval, error) {
	return &FinalizerRemoval{
		Handler: admission.NewHandler(admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (f *FinalizerRemoval) AssignReadyFunc(fn admission.ReadyFunc) {
	f.readyFunc = fn
	f.SetReadyFunc(fn)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (f *FinalizerRemoval) SetCoreInformerFactory(g gardencoreinformers.SharedInformerFactory) {
	shootInformer := g.Core().V1beta1().Shoots()
	f.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs,
		shootInformer.Informer().HasSynced,
	)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (f *FinalizerRemoval) ValidateInitialization() error {
	if f.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	return nil
}

// Admit ensures that finalizers from objects can only be removed if they are not needed anymore.
func (f *FinalizerRemoval) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if f.readyFunc == nil {
		f.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !f.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	var (
		err            error
		newObj, oldObj client.Object
	)

	oldObj, ok := a.GetOldObject().(client.Object)
	if !ok {
		return nil
	}

	newObj, ok = a.GetObject().(client.Object)
	if !ok {
		return nil
	}

	switch a.GetKind().GroupKind() {
	case core.Kind("SecretBinding"):
		binding, ok := a.GetObject().(*core.SecretBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into SecretBinding object")
		}

		// Allow removal of `gardener` finalizer only if the SecretBinding is not used by any shoot.
		if isFinalizerRemoved(oldObj, newObj, gardencorev1beta1.GardenerName) {
			inUse, err := f.isUsedByShoot(binding.Namespace, func(shoot *gardencorev1beta1.Shoot) bool {
				return ptr.Deref(shoot.Spec.SecretBindingName, "") == binding.Name
			})
			if err != nil {
				return apierrors.NewInternalError(fmt.Errorf("error checking if secret binding is in use: %w", err))
			}
			if inUse {
				return admission.NewForbidden(a, fmt.Errorf("finalizer must not be removed - secret binding %s/%s is still in use by at least one shoot", binding.Namespace, binding.Name))
			}
		}
	case security.Kind("CredentialsBinding"):
		binding, ok := a.GetObject().(*security.CredentialsBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into CredentialsBinding object")
		}

		// Allow removal of `gardener` finalizer only if the CredentialsBinding is not used by any shoot.
		if isFinalizerRemoved(oldObj, newObj, gardencorev1beta1.GardenerName) {
			inUse, err := f.isUsedByShoot(binding.Namespace, func(shoot *gardencorev1beta1.Shoot) bool {
				return ptr.Deref(shoot.Spec.CredentialsBindingName, "") == binding.Name
			})
			if err != nil {
				return apierrors.NewInternalError(fmt.Errorf("error checking if credentials binding is in use: %w", err))
			}
			if inUse {
				return admission.NewForbidden(a, fmt.Errorf("finalizer must not be removed - credentials binding %s/%s is still in use by at least one shoot", binding.Namespace, binding.Name))
			}
		}
	case core.Kind("Shoot"):
		shoot, ok := a.GetObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Shoot object")
		}

		// Allow removal of `gardener` finalizer only if the Shoot deletion has completed successfully.
		if isFinalizerRemoved(oldObj, newObj, gardencorev1beta1.GardenerName) && !shootDeletionSucceeded(shoot) {
			return admission.NewForbidden(a, fmt.Errorf("finalizer %q cannot be removed because shoot deletion has not completed successfully yet", core.GardenerName))
		}
	}

	if err != nil {
		return admission.NewForbidden(a, err)
	}
	return nil
}

func (f *FinalizerRemoval) isUsedByShoot(namespace string, inUse func(*gardencorev1beta1.Shoot) bool) (bool, error) {
	shoots, err := f.shootLister.Shoots(namespace).List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("error retrieving shoots: %w", err)
	}

	return slices.ContainsFunc(shoots, inUse), nil
}

func shootDeletionSucceeded(shoot *core.Shoot) bool {
	if len(shoot.Status.TechnicalID) == 0 || shoot.Status.LastOperation == nil {
		return true
	}

	lastOperation := shoot.Status.LastOperation
	return lastOperation.Type == core.LastOperationTypeDelete &&
		lastOperation.State == core.LastOperationStateSucceeded &&
		lastOperation.Progress == 100
}

func isFinalizerRemoved(old, new metav1.Object, finalizerName string) bool {
	var (
		oldFinalizers = sets.New(old.GetFinalizers()...)
		newFinalizer  = sets.New(new.GetFinalizers()...)
	)

	return oldFinalizers.Has(finalizerName) && !newFinalizer.Has(finalizerName)
}
