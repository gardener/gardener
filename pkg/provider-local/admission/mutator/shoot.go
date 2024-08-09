// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
)

// NewShootMutator returns a new instance of a shoot mutator.
func NewShootMutator(mgr manager.Manager) extensionswebhook.Mutator {
	return &shoot{
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type shoot struct {
	decoder runtime.Decoder
}

// Mutate mutates the given shoot object.
func (s *shoot) Mutate(_ context.Context, newObj, oldObj client.Object) error {
	shoot, ok := newObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	var oldShoot *gardencorev1beta1.Shoot
	if oldObj != nil {
		oldShoot, ok = oldObj.(*gardencorev1beta1.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T", oldObj)
		}
	}

	// Skip if specs are matching
	if oldShoot != nil && reflect.DeepEqual(shoot.Spec, oldShoot.Spec) {
		return nil
	}

	// Skip if shoot is in deletion phase
	if shoot.DeletionTimestamp != nil || oldShoot != nil && oldShoot.DeletionTimestamp != nil {
		return nil
	}

	shoot.Spec.Provider.ControlPlaneConfig = &runtime.RawExtension{
		Object: &v1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
			TestValue: ptr.To("filled-by-admission-webhook"),
		},
	}

	return nil
}
