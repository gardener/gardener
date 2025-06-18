// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/test"
	mocketcd "github.com/gardener/gardener/pkg/component/etcd/etcd/mock"
	. "github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("ETCD", func() {
	var (
		ctx    = context.TODO()
		logger logr.Logger

		kubeAPIServerNamespace      = "shoot--foo--bar"
		namePrefix                  = "baz-"
		kubeAPIServerDeploymentName = namePrefix + "kube-apiserver"

		runtimeClient       client.Client
		targetClient        client.Client
		fakeTargetInterface kubernetes.Interface
		fakeSecretsManager  secretsmanager.Interface
		fakeDiscoveryClient *fakeDiscoveryWithServerPreferredResources
	)

	BeforeEach(func() {
		logger = logr.Discard()

		runtimeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		targetClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeDiscoveryClient = &fakeDiscoveryWithServerPreferredResources{}
		fakeTargetInterface = fakekubernetes.NewClientSetBuilder().WithKubernetes(test.NewClientSetWithDiscovery(nil, fakeDiscoveryClient)).WithClient(targetClient).Build()
		fakeSecretsManager = fakesecretsmanager.New(runtimeClient, kubeAPIServerNamespace)
	})

	Context("etcd encryption key secret rotation", func() {
		var (
			namespace1, namespace2                *corev1.Namespace
			secret1, secret2, secret3             *corev1.Secret
			configMap1, configMap2                *corev1.ConfigMap
			endpointSlice1, endpointSlice2        *discoveryv1.EndpointSlice
			deployment1, deployment2, deployment3 *appsv1.Deployment
			kubeAPIServerDeployment               *appsv1.Deployment

			secret1ResourceVersion, secret2ResourceVersion, secret3ResourceVersion             string
			configMap1ResourceVersion, configMap2ResourceVersion                               string
			deployment1ResourceVersion, deployment2ResourceVersion, deployment3ResourceVersion string
			endpointSlice1ResourceVersion, endpointSlice2ResourceVersion                       string
		)

		BeforeEach(func() {
			namespace1 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}}
			namespace2 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}}

			Expect(targetClient.Create(ctx, namespace1)).To(Succeed())
			Expect(targetClient.Create(ctx, namespace2)).To(Succeed())

			secret1 = &corev1.Secret{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
				ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: namespace1.Name},
			}
			secret2 = &corev1.Secret{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
				ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: namespace2.Name},
			}
			secret3 = &corev1.Secret{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
				ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: namespace2.Name, Labels: map[string]string{"credentials.gardener.cloud/key-name": "kube-apiserver-etcd-encryption-key-current"}},
			}

			configMap1 = &corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "configMap1", Namespace: namespace1.Name, Labels: map[string]string{"credentials.gardener.cloud/key-name": "kube-apiserver-etcd-encryption-key-current"}},
			}
			configMap2 = &corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "configMap2", Namespace: namespace2.Name},
			}

			deployment1 = &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
				ObjectMeta: metav1.ObjectMeta{Name: "deployment1", Namespace: namespace1.Name},
			}
			deployment2 = &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
				ObjectMeta: metav1.ObjectMeta{Name: "deployment2", Namespace: namespace1.Name, Labels: map[string]string{"credentials.gardener.cloud/key-name": "kube-apiserver-etcd-encryption-key-current"}},
			}
			deployment3 = &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
				ObjectMeta: metav1.ObjectMeta{Name: "deployment3", Namespace: namespace2.Name},
			}

			endpointSlice1 = &discoveryv1.EndpointSlice{
				TypeMeta:   metav1.TypeMeta{APIVersion: "discovery.k8s.io/v1", Kind: "EndpointSlice"},
				ObjectMeta: metav1.ObjectMeta{Name: "endpointSlice1", Namespace: namespace1.Name},
			}
			endpointSlice2 = &discoveryv1.EndpointSlice{
				TypeMeta:   metav1.TypeMeta{APIVersion: "discovery.k8s.io/v1", Kind: "EndpointSlice"},
				ObjectMeta: metav1.ObjectMeta{Name: "endpointSlice2", Namespace: namespace2.Name, Labels: map[string]string{"credentials.gardener.cloud/key-name": "kube-apiserver-etcd-encryption-key-current"}},
			}

			for _, obj := range []client.Object{
				secret1, secret2, secret3,
				deployment1, deployment2, deployment3,
				configMap1, configMap2,
				endpointSlice1, endpointSlice2,
			} {
				Expect(targetClient.Create(ctx, obj)).To(Succeed())
			}

			for _, obj := range []client.Object{
				secret1, secret2, secret3,
				configMap1, configMap2,
				deployment1, deployment2, deployment3,
				endpointSlice1, endpointSlice2,
			} {
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			}

			secret1ResourceVersion = secret1.ResourceVersion
			secret2ResourceVersion = secret2.ResourceVersion
			secret3ResourceVersion = secret3.ResourceVersion

			configMap1ResourceVersion = configMap1.ResourceVersion
			configMap2ResourceVersion = configMap2.ResourceVersion

			deployment1ResourceVersion = deployment1.ResourceVersion
			deployment2ResourceVersion = deployment2.ResourceVersion
			deployment3ResourceVersion = deployment3.ResourceVersion

			endpointSlice1ResourceVersion = endpointSlice1.ResourceVersion
			endpointSlice2ResourceVersion = endpointSlice2.ResourceVersion

			kubeAPIServerDeployment = &appsv1.Deployment{TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"}, ObjectMeta: metav1.ObjectMeta{Name: kubeAPIServerDeploymentName, Namespace: kubeAPIServerNamespace}}
			Expect(runtimeClient.Create(ctx, kubeAPIServerDeployment)).To(Succeed())
		})

		Describe("#RewriteEncryptedDataAddLabel", func() {
			It("should patch all resources and add the label if not already done", func() {
				Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-etcd-encryption-key-current", Namespace: kubeAPIServerNamespace}})).To(Succeed())

				resources := []string{
					corev1.Resource("secrets").String(),
					corev1.Resource("configmaps").String(),
					appsv1.Resource("deployments").String(),
					discoveryv1.Resource("endpointslices").String(),
				}

				defaultGVKs := []schema.GroupVersionKind{corev1.SchemeGroupVersion.WithKind("Secret")}

				Expect(RewriteEncryptedDataAddLabel(ctx, logger, runtimeClient, fakeTargetInterface, fakeSecretsManager, kubeAPIServerNamespace, kubeAPIServerDeploymentName, resources, resources, defaultGVKs)).To(Succeed())

				for _, obj := range []client.Object{
					secret1, secret2, secret3,
					configMap1, configMap2,
					deployment1, deployment2, deployment3,
					endpointSlice1, endpointSlice2,
				} {
					Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					Expect(obj.GetLabels()).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "kube-apiserver-etcd-encryption-key-current"))
				}

				Expect(secret1.ResourceVersion).NotTo(Equal(secret1ResourceVersion))
				Expect(secret2.ResourceVersion).NotTo(Equal(secret2ResourceVersion))
				Expect(secret3.ResourceVersion).To(Equal(secret3ResourceVersion))

				Expect(configMap1.ResourceVersion).To(Equal(configMap1ResourceVersion))
				Expect(configMap2.ResourceVersion).NotTo(Equal(configMap2ResourceVersion))

				Expect(deployment1.ResourceVersion).NotTo(Equal(deployment1ResourceVersion))
				Expect(deployment2.ResourceVersion).To(Equal(deployment2ResourceVersion))
				Expect(deployment3.ResourceVersion).NotTo(Equal(deployment3ResourceVersion))

				Expect(endpointSlice1.ResourceVersion).NotTo(Equal(endpointSlice1ResourceVersion))
				Expect(endpointSlice2.ResourceVersion).To(Equal(endpointSlice2ResourceVersion))

				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerDeployment), kubeAPIServerDeployment)).To(Succeed())
				Expect(kubeAPIServerDeployment.Annotations).To(HaveKeyWithValue("credentials.gardener.cloud/resources-labeled", "true"))
			})

			It("should not label the resources if the kube-apiserver deployment has the resources-labeled annotation", func() {
				Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-etcd-encryption-key-current", Namespace: kubeAPIServerNamespace}})).To(Succeed())

				resources := []string{
					corev1.Resource("secrets").String(),
					corev1.Resource("configmaps").String(),
					appsv1.Resource("deployments").String(),
					discoveryv1.Resource("endpointslices").String(),
				}

				defaultGVKs := []schema.GroupVersionKind{corev1.SchemeGroupVersion.WithKind("Secret")}

				metav1.SetMetaDataAnnotation(&kubeAPIServerDeployment.ObjectMeta, "credentials.gardener.cloud/resources-labeled", "true")
				Expect(runtimeClient.Update(ctx, kubeAPIServerDeployment)).To(Succeed())

				Expect(RewriteEncryptedDataAddLabel(ctx, logger, runtimeClient, fakeTargetInterface, fakeSecretsManager, kubeAPIServerNamespace, kubeAPIServerDeploymentName, resources, resources, defaultGVKs)).To(Succeed())

				Expect(secret1.ResourceVersion).To(Equal(secret1ResourceVersion))
				Expect(secret2.ResourceVersion).To(Equal(secret2ResourceVersion))
				Expect(secret3.ResourceVersion).To(Equal(secret3ResourceVersion))

				Expect(configMap1.ResourceVersion).To(Equal(configMap1ResourceVersion))
				Expect(configMap2.ResourceVersion).To(Equal(configMap2ResourceVersion))

				Expect(deployment1.ResourceVersion).To(Equal(deployment1ResourceVersion))
				Expect(deployment2.ResourceVersion).To(Equal(deployment2ResourceVersion))
				Expect(deployment3.ResourceVersion).To(Equal(deployment3ResourceVersion))

				Expect(endpointSlice1.ResourceVersion).To(Equal(endpointSlice1ResourceVersion))
				Expect(endpointSlice2.ResourceVersion).To(Equal(endpointSlice2ResourceVersion))
			})
		})

		Describe("#SnapshotETCDAfterRewritingEncryptedData", func() {
			var (
				ctrl     *gomock.Controller
				etcdMain *mocketcd.MockInterface
			)

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				etcdMain = mocketcd.NewMockInterface(ctrl)
			})

			AfterEach(func() {
				ctrl.Finish()
			})

			It("should create a snapshot of ETCD and annotate kube-apiserver accordingly", func() {
				etcdMain.EXPECT().Snapshot(ctx, nil)

				Expect(SnapshotETCDAfterRewritingEncryptedData(ctx, runtimeClient, func(ctx context.Context) error { return etcdMain.Snapshot(ctx, nil) }, kubeAPIServerNamespace, kubeAPIServerDeploymentName)).To(Succeed())

				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerDeployment), kubeAPIServerDeployment)).To(Succeed())
				Expect(kubeAPIServerDeployment.Annotations).To(HaveKeyWithValue("credentials.gardener.cloud/etcd-snapshotted", "true"))
			})
		})

		Describe("#RewriteEncryptedDataRemoveLabel", func() {
			It("should patch all resources and remove the label if not already done", func() {
				metav1.SetMetaDataAnnotation(&kubeAPIServerDeployment.ObjectMeta, "credentials.gardener.cloud/etcd-snapshotted", "true")
				metav1.SetMetaDataAnnotation(&kubeAPIServerDeployment.ObjectMeta, "credentials.gardener.cloud/resources-labeled", "true")
				Expect(runtimeClient.Update(ctx, kubeAPIServerDeployment)).To(Succeed())

				resources := []string{
					corev1.Resource("secrets").String(),
					corev1.Resource("configmaps").String(),
					appsv1.Resource("deployments").String(),
					discoveryv1.Resource("endpointslices").String(),
				}

				defaultGVKs := []schema.GroupVersionKind{corev1.SchemeGroupVersion.WithKind("Secret")}

				Expect(RewriteEncryptedDataRemoveLabel(ctx, logger, runtimeClient, fakeTargetInterface, kubeAPIServerNamespace, kubeAPIServerDeploymentName, resources, resources, defaultGVKs)).To(Succeed())

				for _, obj := range []client.Object{
					secret1, secret2, secret3,
					configMap1, configMap2,
					deployment1, deployment2, deployment3,
					endpointSlice1, endpointSlice2,
				} {
					Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					Expect(obj.GetLabels()).NotTo(HaveKey("credentials.gardener.cloud/key-name"))
				}

				Expect(secret1.ResourceVersion).To(Equal(secret1ResourceVersion))
				Expect(secret2.ResourceVersion).To(Equal(secret2ResourceVersion))
				Expect(secret3.ResourceVersion).NotTo(Equal(secret3ResourceVersion))

				Expect(configMap1.ResourceVersion).NotTo(Equal(configMap1ResourceVersion))
				Expect(configMap2.ResourceVersion).To(Equal(configMap2ResourceVersion))

				Expect(deployment1.ResourceVersion).To(Equal(deployment1ResourceVersion))
				Expect(deployment2.ResourceVersion).NotTo(Equal(deployment2ResourceVersion))
				Expect(deployment3.ResourceVersion).To(Equal(deployment3ResourceVersion))

				Expect(endpointSlice1.ResourceVersion).To(Equal(endpointSlice1ResourceVersion))
				Expect(endpointSlice2.ResourceVersion).NotTo(Equal(endpointSlice2ResourceVersion))

				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerDeployment), kubeAPIServerDeployment)).To(Succeed())
				Expect(kubeAPIServerDeployment.Annotations).NotTo(HaveKey("credentials.gardener.cloud/etcd-snapshotted"))
				Expect(kubeAPIServerDeployment.Annotations).NotTo(HaveKey("credentials.gardener.cloud/resources-labeled"))
			})
		})
	})

	Describe("GetResourcesForRewrite", func() {
		It("should return the correct GVK list when the resources to encrypt and encrypted resources are equal (encryption key rotation)", func() {
			var (
				resources = []string{
					"crontabs.stable.example.com",
					"managedresources.resources.gardener.cloud",
					"configmaps",
					"deployments.apps",
				}

				defaultGVKs = []schema.GroupVersionKind{corev1.SchemeGroupVersion.WithKind("Secret")}
			)

			list, message, err := GetResourcesForRewrite(fakeDiscoveryClient, resources, resources, defaultGVKs)
			Expect(err).NotTo(HaveOccurred())
			Expect(message).To(Equal("Objects requiring to be rewritten after ETCD encryption key rotation"))
			Expect(list).To(ConsistOf(
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
				schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronTab"},
				schema.GroupVersionKind{Group: "resources.gardener.cloud", Version: "v1alpha1", Kind: "ManagedResource"},
				schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			))
		})

		It("should return the correct GVK list when not all resources are part from resourece list", func() {
			var (
				resourcesToEncrypt = []string{
					"crontabs.stable.example.com",
					"baz",
					"configmaps",
					"foo.bar",
				}

				encryptedResources = []string{
					"baz",
					"foo.bar",
					"crontabs.stable.example.com",
					"configmaps",
				}

				defaultGVKs = []schema.GroupVersionKind{corev1.SchemeGroupVersion.WithKind("Secret")}
			)

			list, message, err := GetResourcesForRewrite(fakeDiscoveryClient, resourcesToEncrypt, encryptedResources, defaultGVKs)
			Expect(err).NotTo(HaveOccurred())
			Expect(message).To(Equal("Objects requiring to be rewritten after ETCD encryption key rotation"))
			Expect(list).To(ConsistOf(
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
				schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronTab"},
			))
		})

		It("should return the correct GVK list for the modified resources when the resources to encrypt and encrypted resources are not equal", func() {
			var (
				resourcesToEncrypt = []string{
					"crontabs.stable.example.com",
					"configmaps",
					"managedresources.resources.gardener.cloud",
					"deployments.apps",
				}

				encryptedResources = []string{
					"managedresources.resources.gardener.cloud",
					"deployments.apps",
					"services",
					"cronbars.stable.example.com",
				}

				defaultGVKs = []schema.GroupVersionKind{corev1.SchemeGroupVersion.WithKind("Secret")}
			)

			list, message, err := GetResourcesForRewrite(fakeDiscoveryClient, resourcesToEncrypt, encryptedResources, defaultGVKs)
			Expect(err).NotTo(HaveOccurred())
			Expect(message).To(Equal("Objects requiring to be rewritten after modification of encryption config"))
			Expect(list).To(ConsistOf(
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
				schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronTab"},
				schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronBar"},
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
			))
		})
	})
})

type fakeDiscoveryWithServerPreferredResources struct {
	*fakediscovery.FakeDiscovery
}

func (c *fakeDiscoveryWithServerPreferredResources) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "configmaps",
					Namespaced: true,
					Group:      corev1.SchemeGroupVersion.Group,
					Version:    corev1.SchemeGroupVersion.Version,
					Kind:       "ConfigMap",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
				},
				{
					Name:       "services",
					Namespaced: true,
					Group:      corev1.SchemeGroupVersion.Group,
					Version:    corev1.SchemeGroupVersion.Version,
					Kind:       "Service",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					ShortNames: []string{"svc"},
				},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "daemonsets",
					SingularName: "daemonset",
					Namespaced:   true,
					Group:        appsv1.SchemeGroupVersion.Group,
					Version:      appsv1.SchemeGroupVersion.Version,
					Kind:         "DaemonSet",
					Verbs:        metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					ShortNames:   []string{"ds"},
				},
				{
					Name:         "deployments",
					SingularName: "deployment",
					Namespaced:   true,
					Group:        "",
					Version:      "",
					Kind:         "Deployment",
					Verbs:        metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					ShortNames:   []string{"deploy"},
				},
			},
		},
		{
			GroupVersion: "discovery/v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "endpointslices",
					SingularName: "endpointslice",
					Namespaced:   true,
					Group:        discoveryv1.SchemeGroupVersion.Group,
					Version:      discoveryv1.SchemeGroupVersion.Version,
					Kind:         "EndpointSlice",
					Verbs:        metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
				},
			},
		},
		{
			GroupVersion: "resources.gardener.cloud/v1alpha1",
			APIResources: []metav1.APIResource{
				{
					Name:       "managedresources",
					Namespaced: true,
					Group:      "resources.gardener.cloud",
					Version:    "v1alpha1",
					Kind:       "ManagedResource",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
				},
			},
		},
		{
			GroupVersion: "stable.example.com/v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "crontabs",
					SingularName: "crontab",
					Namespaced:   true,
					Group:        "stable.example.com",
					Version:      "v1",
					Kind:         "CronTab",
					Verbs:        metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
				},
				{
					Name:         "cronbars",
					SingularName: "cronbar",
					Namespaced:   true,
					Group:        "stable.example.com",
					Version:      "v1",
					Kind:         "CronBar",
					Verbs:        metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
				},
			},
		},
	}, nil
}
