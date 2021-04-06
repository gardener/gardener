// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed"
	"github.com/gardener/gardener/pkg/logger"
	mockcorev1 "github.com/gardener/gardener/pkg/mock/client-go/core/v1"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/client-go/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8scoreinformers "k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("SeedReconciler", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		var (
			cl          *mockclient.MockClient
			k           *mockkubernetes.MockInterface
			corev1If    *mockcorev1.MockCoreV1Interface
			namespaceIf *mockcorev1.MockNamespaceInterface
			secretIf    *mockcorev1.MockSecretInterface

			control reconcile.Reconciler
			cm      *fakeclientmap.ClientMap

			secrets   []*corev1.Secret
			seed      *gardencorev1beta1.Seed
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			cl = mockclient.NewMockClient(ctrl)
			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seed",
				},
			}
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: gardenerutils.ComputeGardenNamespace(seed.Name),
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed")),
					},
				},
			}
		})

		JustBeforeEach(func() {
			gardenInformerFactory := gardencoreinformers.NewSharedInformerFactory(nil, 0)
			Expect(gardenInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

			coreInformerFactory := k8scoreinformers.NewSharedInformerFactory(nil, 0)
			for _, s := range secrets {
				secret := s
				Expect(coreInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(secret)).To(Succeed())
			}

			fakeGardenClientSet := fakeclientset.NewClientSetBuilder().
				WithKubernetes(k).
				WithClient(cl).
				Build()
			cm = fakeclientmap.NewClientMapBuilder().WithClientSetForKey(keys.ForGarden(), fakeGardenClientSet).Build()

			control = NewDefaultControl(cm, coreInformerFactory.Core().V1().Secrets().Lister(), gardenInformerFactory.Core().V1beta1().Seeds().Lister())
		})

		Context("when seed exists", func() {
			var (
				seedNamespace *corev1.Namespace

				addedSecret, oldSecret, newSecret, deletedSecret *corev1.Secret
			)

			BeforeEach(func() {
				cl = mockclient.NewMockClient(ctrl)
				k = mockkubernetes.NewMockInterface(ctrl)
				corev1If = mockcorev1.NewMockCoreV1Interface(ctrl)
				namespaceIf = mockcorev1.NewMockNamespaceInterface(ctrl)
				secretIf = mockcorev1.NewMockSecretInterface(ctrl)

				k.EXPECT().CoreV1().Return(corev1If).AnyTimes()
				corev1If.EXPECT().Secrets(gomock.Any()).Return(secretIf).AnyTimes()
				corev1If.EXPECT().Namespaces().Return(namespaceIf).AnyTimes()

				seedNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gardenerutils.ComputeGardenNamespace(seed.Name)}}

				oldSecret = createSecret("existing", seedNamespace.Name, "old", "role", []byte("data"))
				newSecret = createSecret("existing", v1beta1constants.GardenNamespace, "foo", "role", []byte("bar"))
				addedSecret = createSecret("new", v1beta1constants.GardenNamespace, "foo", "role", []byte("bar"))
				deletedSecret = createSecret("stale", seedNamespace.Name, "foo", "role", []byte("bar"))
				secrets = []*corev1.Secret{addedSecret, newSecret, oldSecret, deletedSecret}
			})

			It("should update the namespace and sync secrets", func() {
				// cause namespace update
				cl.EXPECT().Get(context.Background(), kutil.Key(gardenerutils.ComputeGardenNamespace(seed.Name)), gomock.AssignableToTypeOf(&corev1.Namespace{}))
				cl.EXPECT().Update(context.Background(), gomock.AssignableToTypeOf(&corev1.Namespace{}))

				// cause secret update
				cl.EXPECT().Create(context.Background(), copySecretWithNamespace(newSecret, seedNamespace.Name)).Return(apierrors.NewAlreadyExists(corev1.Resource("secrets"), ""))

				// expect update for existing secret
				secretIf.EXPECT().Get(context.Background(), oldSecret.Name, kubernetes.DefaultGetOptions()).Return(oldSecret, nil)
				secretIf.EXPECT().Update(context.Background(), copySecretWithNamespace(newSecret, seedNamespace.Name), kubernetes.DefaultUpdateOptions()).Return(nil, nil)

				// expect create for non existing secret
				cl.EXPECT().Create(context.Background(), copySecretWithNamespace(addedSecret, seedNamespace.Name)).Return(nil)

				// expect deletion for deleted secret in Garden namespace
				cl.EXPECT().Delete(context.Background(), deletedSecret).Return(nil)

				result, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("when seed is new", func() {
			BeforeEach(func() {
				secrets = []*corev1.Secret{
					createSecret("1", v1beta1constants.GardenNamespace, "foo", "role", []byte("bar")),
					createSecret("2", v1beta1constants.GardenNamespace, "foo", "role", []byte("bar")),
					createSecret("3", v1beta1constants.GardenNamespace, "foo", v1beta1constants.GardenRoleMonitoring, []byte("bar")),
				}
			})

			It("should create and copy assets", func() {
				cl.EXPECT().Get(context.Background(), kutil.Key(gardenerutils.ComputeGardenNamespace(seed.Name)), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				cl.EXPECT().Create(context.Background(), namespace).Return(nil)
				cl.EXPECT().Create(context.Background(), copySecretWithNamespace(secrets[0], namespace.Name)).Return(nil)
				cl.EXPECT().Create(context.Background(), copySecretWithNamespace(secrets[1], namespace.Name)).Return(nil)

				result, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should not create and copy assets if seed cannot be found", func() {
				defer test.WithVar(&logger.Logger, logger.NewNopLogger())()
				result, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: kutil.Key("Gone")})
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})

func copySecretWithNamespace(secret *corev1.Secret, namespace string) *corev1.Secret {
	s := secret.DeepCopy()
	s.SetNamespace(namespace)
	return s
}

func createSecret(name, namespace, key, role string, data []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				v1beta1constants.GardenRole: role,
			},
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			key: data,
		},
	}
}
