// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package predicate

import (
	"context"
	"encoding/json"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockcache "github.com/gardener/gardener/pkg/mock/controller-runtime/cache"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	contextutil "github.com/gardener/gardener/pkg/utils/context"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("Predicate", func() {
	var (
		ctrl  *gomock.Controller
		c     *mockclient.MockClient
		cache *mockcache.MockCache
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		cache = mockcache.NewMockCache(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#HasType", func() {
		var (
			extensionType string
			object        client.Object
			createEvent   event.CreateEvent
			updateEvent   event.UpdateEvent
			deleteEvent   event.DeleteEvent
			genericEvent  event.GenericEvent
		)

		BeforeEach(func() {
			extensionType = "extension-type"
			object = &extensionsv1alpha1.Extension{
				Spec: extensionsv1alpha1.ExtensionSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: extensionType,
					},
				},
			}
			createEvent = event.CreateEvent{
				Object: object,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: object,
				ObjectNew: object,
			}
			deleteEvent = event.DeleteEvent{
				Object: object,
			}
			genericEvent = event.GenericEvent{
				Object: object,
			}
		})

		It("should match the type", func() {
			predicate := HasType(extensionType)

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeTrue())
		})

		It("should not match the type", func() {
			predicate := HasType("anotherType")

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeFalse())
		})
	})

	Describe("#HasName", func() {
		var (
			name         string
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}

			createEvent = event.CreateEvent{
				Object: configMap,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: configMap,
				ObjectNew: configMap,
			}
			deleteEvent = event.DeleteEvent{
				Object: configMap,
			}
			genericEvent = event.GenericEvent{
				Object: configMap,
			}
		})

		It("should match the name", func() {
			predicate := HasName(name)

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeTrue())
		})

		It("should not match the name", func() {
			predicate := HasName("anotherName")

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeFalse())
		})
	})

	const (
		extensionType = "extension-type"
		version       = "1.18"
	)

	Describe("#ClusterShootProviderType", func() {
		var decoder runtime.Decoder

		BeforeEach(func() {
			decoder = extensionscontroller.NewGardenDecoder()
		})

		It("should match the type", func() {
			var (
				predicate                                           = ClusterShootProviderType(decoder, extensionType)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(extensionType, version, nil)
			)

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeTrue())
		})

		It("should not match the type", func() {
			var (
				predicate                                           = ClusterShootProviderType(decoder, extensionType)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents("other-extension-type", version, nil)
			)

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeFalse())
		})
	})

	Describe("#ClusterShootKubernetesVersionForCSIMigrationAtLeast", func() {
		var decoder runtime.Decoder

		BeforeEach(func() {
			decoder = extensionscontroller.NewGardenDecoder()
		})

		It("should match the minimum kubernetes version", func() {
			var (
				predicate                                           = ClusterShootKubernetesVersionForCSIMigrationAtLeast(decoder, version)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(extensionType, version, nil)
			)

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeTrue())
		})

		It("should not match the minimum kubernetes version", func() {
			var (
				predicate                                           = ClusterShootKubernetesVersionForCSIMigrationAtLeast(decoder, version)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(extensionType, "1.17", nil)
			)

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeFalse())
		})

		It("should not match minimum kubernetes version due to overwrite", func() {
			var (
				predicate                                           = ClusterShootKubernetesVersionForCSIMigrationAtLeast(decoder, version)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(extensionType, "1.17", pointer.String("1.17"))
			)

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeTrue())
		})
	})

	Describe("#ShootNotFailed", func() {
		const name = "shoot--foo--bar"

		var (
			ctx            = contextutil.FromStopChannel(context.TODO().Done())
			mapper         *shootNotFailedMapper
			infrastructure *extensionsv1alpha1.Infrastructure
			e              event.GenericEvent
		)

		BeforeEach(func() {
			mapper = &shootNotFailedMapper{log: Log.WithName("shoot-not-failed")}
			gomega.Expect(mapper.InjectStopChannel(context.TODO().Done())).To(gomega.Succeed())
			gomega.Expect(mapper.InjectCache(cache)).To(gomega.Succeed())
			gomega.Expect(mapper.InjectClient(c)).To(gomega.Succeed())

			infrastructure = &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: name}}
			e = event.GenericEvent{
				Object: infrastructure,
			}
		})

		It("should return true because shoot has no last operation", func() {
			cache.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)

			meta := &metav1.ObjectMeta{Generation: 1}
			status := &gardencorev1beta1.ShootStatus{
				ObservedGeneration: 1,
			}

			cluster := computeClusterWithShoot(name, meta, nil, status)
			c.EXPECT().Get(gomock.AssignableToTypeOf(ctx), kutil.Key(name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *extensionsv1alpha1.Cluster) error {
				cluster.DeepCopyInto(actual)
				return nil
			})

			gomega.Expect(mapper.Map(e)).To(gomega.BeTrue())
		})

		It("should return true because shoot last operation state is not failed", func() {
			cache.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)

			meta := &metav1.ObjectMeta{Generation: 1}
			status := &gardencorev1beta1.ShootStatus{
				ObservedGeneration: 1,
				LastOperation:      &gardencorev1beta1.LastOperation{},
			}

			cluster := computeClusterWithShoot(name, meta, nil, status)
			c.EXPECT().Get(gomock.AssignableToTypeOf(ctx), kutil.Key(name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *extensionsv1alpha1.Cluster) error {
				cluster.DeepCopyInto(actual)
				return nil
			})

			gomega.Expect(mapper.Map(e)).To(gomega.BeTrue())
		})

		It("should return false because shoot is failed", func() {
			cache.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)

			meta := &metav1.ObjectMeta{Generation: 1}
			status := &gardencorev1beta1.ShootStatus{
				ObservedGeneration: 1,
				LastOperation:      &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed},
			}

			cluster := computeClusterWithShoot(name, meta, nil, status)
			c.EXPECT().Get(gomock.AssignableToTypeOf(ctx), kutil.Key(name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *extensionsv1alpha1.Cluster) error {
				cluster.DeepCopyInto(actual)
				return nil
			})

			gomega.Expect(mapper.Map(e)).To(gomega.BeFalse())
		})

		It("should return true because shoot is failed but observed generation is outdated", func() {
			cache.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)

			meta := &metav1.ObjectMeta{Generation: 2}
			status := &gardencorev1beta1.ShootStatus{
				ObservedGeneration: 1,
				LastOperation:      &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed},
			}

			cluster := computeClusterWithShoot(name, meta, nil, status)
			c.EXPECT().Get(gomock.AssignableToTypeOf(ctx), kutil.Key(name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *extensionsv1alpha1.Cluster) error {
				cluster.DeepCopyInto(actual)
				return nil
			})

			gomega.Expect(mapper.Map(e)).To(gomega.BeTrue())
		})

		It("should return true because the resource is in the garden namespace", func() {
			e = event.GenericEvent{
				Object: &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: v1beta1constants.GardenNamespace}},
			}

			gomega.Expect(mapper.Map(e)).To(gomega.BeTrue())
		})
	})
})

func computeClusterWithShoot(name string, shootMeta *metav1.ObjectMeta, shootSpec *gardencorev1beta1.ShootSpec, shootStatus *gardencorev1beta1.ShootStatus) *extensionsv1alpha1.Cluster {
	shoot := &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Shoot",
		},
	}

	if shootMeta != nil {
		shoot.ObjectMeta = *shootMeta
	}
	if shootSpec != nil {
		shoot.Spec = *shootSpec
	}
	if shootStatus != nil {
		shoot.Status = *shootStatus
	}

	shootJSON, err := json.Marshal(shoot)
	gomega.Expect(err).To(gomega.Succeed())

	return &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			Shoot: runtime.RawExtension{Raw: shootJSON},
		},
	}
}

func computeEvents(extensionType, kubernetesVersion string, kubernetesVersionOverwriteAnnotation *string) (event.CreateEvent, event.UpdateEvent, event.DeleteEvent, event.GenericEvent) {
	spec := gardencorev1beta1.ShootSpec{
		Provider: gardencorev1beta1.Provider{
			Type: extensionType,
		},
		Kubernetes: gardencorev1beta1.Kubernetes{
			Version: kubernetesVersion,
		},
	}

	var meta *metav1.ObjectMeta
	if kubernetesVersionOverwriteAnnotation != nil {
		meta = &metav1.ObjectMeta{
			Annotations: map[string]string{
				"alpha.csimigration.shoot.extensions.gardener.cloud/kubernetes-version": *kubernetesVersionOverwriteAnnotation,
			},
		}
	}

	cluster := computeClusterWithShoot("", meta, &spec, nil)

	return event.CreateEvent{Object: cluster},
		event.UpdateEvent{ObjectOld: cluster, ObjectNew: cluster},
		event.DeleteEvent{Object: cluster},
		event.GenericEvent{Object: cluster}
}
