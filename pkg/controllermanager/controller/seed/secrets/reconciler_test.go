// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/secrets"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	mockcorev1 "github.com/gardener/gardener/third_party/mock/client-go/core/v1"
	mockclientgo "github.com/gardener/gardener/third_party/mock/client-go/kubernetes"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Reconciler", func() {
	var (
		ctrl *gomock.Controller

		gardenRoleReq = utils.MustNewRequirement(v1beta1constants.GardenRole, selection.Exists)
		labelSelector = client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(gardenRoleReq).Add(gardenerutils.NoControlPlaneSecretsReq)}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		var (
			cl          *mockclient.MockClient
			k           *mockclientgo.MockInterface
			corev1If    *mockcorev1.MockCoreV1Interface
			namespaceIf *mockcorev1.MockNamespaceInterface
			secretIf    *mockcorev1.MockSecretInterface

			control reconcile.Reconciler

			seed      *gardencorev1beta1.Seed
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			cl = mockclient.NewMockClient(ctrl)
			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seed",
					UID:  "abcdef",
				},
			}
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: gardenerutils.ComputeGardenNamespace(seed.Name),
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed")),
					},
					Labels: map[string]string{"gardener.cloud/role": "seed"},
				},
			}
		})

		JustBeforeEach(func() {
			control = &Reconciler{
				Client:          cl,
				GardenNamespace: "garden",
			}
		})

		It("should fail if get namespace fails", func() {
			cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(errors.New("fake"))

			_, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
			Expect(err).To(MatchError(ContainSubstring("fake")))
		})

		It("should fail if get namespace fails", func() {
			cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Seed, _ ...client.GetOption) error {
				*obj = *seed
				return nil
			})

			cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespace.Name}, gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(errors.New("fake"))

			_, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
			Expect(err).To(MatchError(ContainSubstring("fake")))
		})

		Context("when seed exists", func() {
			var (
				addedSecret, oldSecret, deletedSecret *corev1.Secret
			)

			BeforeEach(func() {
				cl = mockclient.NewMockClient(ctrl)
				k = mockclientgo.NewMockInterface(ctrl)
				corev1If = mockcorev1.NewMockCoreV1Interface(ctrl)
				namespaceIf = mockcorev1.NewMockNamespaceInterface(ctrl)
				secretIf = mockcorev1.NewMockSecretInterface(ctrl)

				k.EXPECT().CoreV1().Return(corev1If).AnyTimes()
				corev1If.EXPECT().Secrets(gomock.Any()).Return(secretIf).AnyTimes()
				corev1If.EXPECT().Namespaces().Return(namespaceIf).AnyTimes()

				oldSecret = createSecret("existing", namespace.Name, "old", []byte("data"))
				addedSecret = createSecret("new", v1beta1constants.GardenNamespace, "foo", []byte("bar"))
				deletedSecret = createSecret("stale", namespace.Name, "foo", []byte("bar"))

				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Seed, _ ...client.GetOption) error {
					*obj = *seed
					return nil
				})
			})

			It("should fail if namespace exists and has no ownerReference", func() {
				namespace.SetOwnerReferences(nil)
				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: gardenerutils.ComputeGardenNamespace(seed.Name)}, gomock.AssignableToTypeOf(&corev1.Namespace{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, ns *corev1.Namespace, _ ...client.GetOption) error {
						namespace.DeepCopyInto(ns)
						return nil
					})

				_, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).To(MatchError(ContainSubstring("not controlled by")))
			})

			It("should fail if namespace exists and is not controlled by seed", func() {
				owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "boss", UID: "12345"}}
				namespace.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(owner, corev1.SchemeGroupVersion.WithKind("ConfigMap"))})
				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: gardenerutils.ComputeGardenNamespace(seed.Name)}, gomock.AssignableToTypeOf(&corev1.Namespace{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, ns *corev1.Namespace, _ ...client.GetOption) error {
						namespace.DeepCopyInto(ns)
						return nil
					})

				_, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).To(MatchError(ContainSubstring("not controlled by")))
			})

			It("should sync secrets if namespace exists and is controlled by seed", func() {
				cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), labelSelector).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					(&corev1.SecretList{Items: []corev1.Secret{*oldSecret, *addedSecret}}).DeepCopyInto(list)
					return nil
				})

				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: gardenerutils.ComputeGardenNamespace(seed.Name)}, gomock.AssignableToTypeOf(&corev1.Namespace{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, ns *corev1.Namespace, _ ...client.GetOption) error {
						namespace.DeepCopyInto(ns)
						return nil
					})

				// expect update for existing secret
				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace.Name, Name: oldSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{}))
				cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any())

				// expect create for non existing secret
				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace.Name, Name: addedSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				cl.EXPECT().Create(gomock.Any(), copySecretWithNamespace(addedSecret, namespace.Name))

				// expect deletion for deleted secret in Garden namespace
				cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespace.Name), labelSelector).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					(&corev1.SecretList{Items: []corev1.Secret{*deletedSecret}}).DeepCopyInto(list)
					return nil
				})
				cl.EXPECT().Delete(gomock.Any(), deletedSecret)

				result, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("when seed is new", func() {
			It("should fail if namespace exists but not in the cache", func() {
				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Seed, _ ...client.GetOption) error {
					*obj = *seed
					return nil
				})

				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespace.Name}, gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				cl.EXPECT().Create(gomock.Any(), namespace).Return(errors.New("fake"))

				_, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).To(MatchError(ContainSubstring("fake")))
			})

			It("should create namespace and sync secrets if namespace does not exists", func() {
				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Seed, _ ...client.GetOption) error {
					*obj = *seed
					return nil
				})

				var (
					secret1 = createSecret("1", v1beta1constants.GardenNamespace, "foo", []byte("bar"))
					secret2 = createSecret("2", v1beta1constants.GardenNamespace, "foo", []byte("bar"))
				)

				cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), labelSelector).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					(&corev1.SecretList{Items: []corev1.Secret{*secret1, *secret2}}).DeepCopyInto(list)
					return nil
				})

				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespace.Name}, gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				cl.EXPECT().Create(gomock.Any(), namespace)
				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace.Name, Name: secret1.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				cl.EXPECT().Create(gomock.Any(), copySecretWithNamespace(secret1, namespace.Name))
				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace.Name, Name: secret2.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				cl.EXPECT().Create(gomock.Any(), copySecretWithNamespace(secret2, namespace.Name))

				cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(namespace.Name), labelSelector).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					(&corev1.SecretList{}).DeepCopyInto(list)
					return nil
				})

				result, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should not create and copy assets if seed cannot be found", func() {
				cl.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: seed.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

				result, err := control.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
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

func createSecret(name, namespace, key string, data []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				v1beta1constants.GardenRole: "role",
			},
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			key: data,
		},
	}
}
