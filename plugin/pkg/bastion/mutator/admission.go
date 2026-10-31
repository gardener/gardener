// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameBastionMutator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// BastionMutator contains the garden core client and admission handler.
type BastionMutator struct {
	*admission.Handler

	coreClient gardencoreclientset.Interface
	readyFunc  admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreClientSet(&BastionMutator{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new BastionMutator admission plugin.
func New() (*BastionMutator, error) {
	return &BastionMutator{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (m *BastionMutator) AssignReadyFunc(f admission.ReadyFunc) {
	m.readyFunc = f
	m.SetReadyFunc(f)
}

// SetCoreClientSet sets the garden core clientset.
func (m *BastionMutator) SetCoreClientSet(c gardencoreclientset.Interface) {
	m.coreClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (m *BastionMutator) ValidateInitialization() error {
	if m.coreClient == nil {
		return errors.New("missing garden core client")
	}
	return nil
}

var _ admission.MutationInterface = (*BastionMutator)(nil)

// Admit mutates the given bastion against the shoot that it references.
func (m *BastionMutator) Admit(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Bastion
	if a.GetKind().GroupKind() != operations.Kind("Bastion") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	// Convert object to Bastion
	bastion, ok := a.GetObject().(*operations.Bastion)
	if !ok {
		return apierrors.NewBadRequest("could not convert object to Bastion")
	}

	// The shoot reference is validated by the BastionValidator admission plugin. If it is not set here, there is
	// nothing to mutate against, so we skip and let the validating plugin reject the request.
	shootName := bastion.Spec.ShootRef.Name
	if shootName == "" {
		return nil
	}

	shoot, err := m.coreClient.CoreV1beta1().Shoots(bastion.Namespace).Get(ctx, shootName, metav1.GetOptions{})
	if err != nil {
		// The shoot existence is validated by the BastionValidator admission plugin. If it does not exist, there is
		// nothing to mutate against, so we skip and let the validating plugin reject the request.
		if apierrors.IsNotFound(err) {
			return nil
		}
		return apierrors.NewInternalError(fmt.Errorf("could not get shoot %s/%s: %v", bastion.Namespace, shootName, err))
	}

	// update bastion
	bastion.Spec.SeedName = shoot.Spec.SeedName
	bastion.Spec.ProviderType = &shoot.Spec.Provider.Type

	if userInfo := a.GetUserInfo(); a.GetOperation() == admission.Create && userInfo != nil {
		metav1.SetMetaDataAnnotation(&bastion.ObjectMeta, v1beta1constants.GardenCreatedBy, userInfo.GetName())
	}

	// ensure bastions are cleaned up when shoots are deleted
	ownerRef := *metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	bastion.OwnerReferences = kubernetes.MergeOwnerReferences(bastion.OwnerReferences, ownerRef)

	return nil
}
