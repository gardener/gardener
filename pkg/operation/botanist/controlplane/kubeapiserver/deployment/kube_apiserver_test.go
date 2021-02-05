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

package deployment_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockcorev1 "github.com/gardener/gardener/pkg/mock/client-go/core/v1"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/client-go/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mock "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	mockio "github.com/gardener/gardener/pkg/mock/go/io"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubeapiserver/deployment"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	fakerestclient "k8s.io/client-go/rest/fake"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	fakeError = fmt.Errorf("fake error")

	// mock
	mockGardenClient  *mockclient.MockClient
	mockSeedInterface *mock.MockInterface
	mockSeedClient    *mockclient.MockClient

	// default configuration
	defaultSeedNamespace                     = "shoot--foo--bar"
	defaultGardenNamespace                   = "garden-foo"
	defaultShootK8sVersion                   = "1.20.0" // tests for other Kubernetes versions also implemented
	defaultSeedK8sVersion                    = defaultShootK8sVersion
	_, defaultServiceNetwork, _              = net.ParseCIDR("100.64.0.0/13")
	_, defaultPodNetwork, _                  = net.ParseCIDR("100.96.0.0/11")
	defaultMinNodeCount                      = int32(1)
	defaultMaxNodeCount                      = int32(2)
	defaultShootOutOfClusterAPIServerAddress = "api.internal-domain"
	defaultShootAPIServerClusterIP           = "100.96.1.0"
	defaultHealthCheckToken                  = "tokenizer"

	apiServerImageName                       = "apiserver-image"
	alpineIptablesImageName                  = "alpine-iptables-image"
	vpnSeedImageName                         = "vpn-seed-image"
	konnectivityServerTunnelImageName        = "konnectivity-image"
	apiServerProxyPodMutatorWebhookImageName = "apiserverproxy-image"

	// secret checksums
	checksumCA                           = "1"
	checksumCaFrontProxy                 = "12"
	checksumTLSServer                    = "123"
	checksumKubeAggregator               = "1234"
	checksumKubeAPIServerKubelet         = "12345"
	checksumStaticToken                  = "123456"
	checksumServiceAccountKey            = "1234567"
	checksumCAEtcd                       = "12345678"
	checksumETCDClientTLS                = "123456789"
	checksumBasicAuth                    = "12345678910"
	checksumKonnectivityServer           = "12345678911"
	checksumKonnectivityServerKubeconfig = "12345678912"
	checksumVPNSeed                      = "12345678913"
	checksumVPNSeedTLSAuth               = "12345678914"
	checksumETCDEncryptionSecret         = "12345678915"
	checksumKonnectivityServerClientTLS  = "12345678916"
)

var _ = Describe("KubeAPIServer", func() {
	var (
		ctx         = context.TODO()
		waiter      *retryfake.Ops
		cleanupFunc func()
		ctrl        *gomock.Controller

		kubeAPIServer  KubeAPIServer
		valuesProvider KubeAPIServerValuesProvider
	)

	BeforeEach(func() {
		// mock
		ctrl = gomock.NewController(GinkgoT())
		mockGardenClient = mockclient.NewMockClient(ctrl)
		mockSeedClient = mockclient.NewMockClient(ctrl)
		mockSeedInterface = mock.NewMockInterface(ctrl)

		// retry
		waiter = &retryfake.Ops{MaxAttempts: 1}
		cleanupFunc = test.WithVars(
			&retry.Until, waiter.Until,
			&retry.UntilTimeout, waiter.UntilTimeout,
		)

		// deploy
		mockSeedInterface.EXPECT().Client().Return(mockSeedClient).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
		cleanupFunc()
	})

	Describe("#Destroy", func() {
		It("should succeed", func() {
			expected := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-apiserver",
					Namespace: defaultSeedNamespace,
				},
			}
			deleteOptions := []client.DeleteOption{
				client.PropagationPolicy("Foreground"),
				client.GracePeriodSeconds(60),
			}

			mockSeedClient.EXPECT().Delete(ctx, expected, deleteOptions).Return(nil)
			kubeAPIServer, _ = NewAPIServerBuilder().Build()
			err := kubeAPIServer.Destroy(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#Wait", func() {
		var (
			apiServerDeployment appsv1.Deployment

			namespace               = "shoot-ns"
			apiServerDeploymentName = "kube-apiserver"
			apiServerDeploymentUID  = "default-UID"
			labels                  = map[string]string{"foo": "bar"}

			listOptions = []interface{}{
				client.InNamespace(namespace),
				client.MatchingLabels(labels),
			}

			// get logs
			mockPodInterface *mockcorev1.MockPodInterface
			body             *mockio.MockReadCloser
			podLogsClient    *http.Client
		)

		BeforeEach(func() {
			kubeAPIServer, _ = NewAPIServerBuilder().Build()

			// client for retry function
			mockSeedInterface.EXPECT().DirectClient().Return(mockSeedClient)

			apiServerDeployment = appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:       apiServerDeploymentName,
					Namespace:  namespace,
					Generation: 1,
					UID:        types.UID(apiServerDeploymentUID),
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
				},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: 1,
				},
			}
		})

		It("should succeed", func() {
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, actual *appsv1.Deployment) error {
					*actual = apiServerDeployment
					return nil
				})

			err := kubeAPIServer.Wait(ctx)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error - cannot get kube-apiserver", func() {
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeError)
			err := kubeAPIServer.Wait(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(fakeError))
		})

		Describe("Tests that read pod logs", func() {
			var (
				apiServerReplicaSet = appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
					Name:              "kube-apiserver-replicaset",
					UID:               "replicaset1",
					CreationTimestamp: metav1.Now(),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       apiServerDeploymentName,
						UID:        types.UID(apiServerDeploymentUID),
					}},
				}}

				apiServerPod = corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name:              "pod1",
					Namespace:         namespace,
					UID:               "pod1",
					CreationTimestamp: metav1.Now(),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       apiServerReplicaSet.Name,
						UID:        apiServerReplicaSet.UID,
					}},
				}}

				logs = []byte("logs")
			)

			BeforeEach(func() {
				// second invocation, after retry failed
				mockSeedInterface.EXPECT().DirectClient().Return(mockSeedClient)

				// NewestPodForDeployment - determine newest replica set
				mockSeedClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), listOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
					*list = appsv1.ReplicaSetList{
						Items: []appsv1.ReplicaSet{
							apiServerReplicaSet,
						},
					}
					return nil
				})

				// NewestPodForDeployment - determine newest pod
				mockSeedClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions...).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{apiServerPod}}
					return nil
				})

				// MostRecentCompleteLogs
				mockKubernetesInterface := mockkubernetes.NewMockInterface(ctrl)
				mockCoreV1Interface := mockcorev1.NewMockCoreV1Interface(ctrl)
				mockPodInterface = mockcorev1.NewMockPodInterface(ctrl)

				// mock k.seedClient.Kubernetes().CoreV1().Pods(newestPod.Namespace)
				mockSeedInterface.EXPECT().Kubernetes().Return(mockKubernetesInterface)
				mockKubernetesInterface.EXPECT().CoreV1().Return(mockCoreV1Interface)
				mockCoreV1Interface.EXPECT().Pods(namespace).Return(mockPodInterface)

				// client for pod logs
				body = mockio.NewMockReadCloser(ctrl)
				podLogsClient = fakerestclient.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
				})

				gomock.InOrder(
					mockPodInterface.EXPECT().GetLogs(apiServerPod.Name, &corev1.PodLogOptions{
						Container: "kube-apiserver",
						Previous:  false,
						TailLines: pointer.Int64Ptr(10),
					}).Return(rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, podLogsClient)),
					body.EXPECT().Read(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
						copy(data, logs)
						return len(logs), io.EOF
					}),
					body.EXPECT().Close(),
				)
			})

			It("should return error - kube-apiserver not observed at latest generation", func() {
				observedGeneration := int64(2)
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, actual *appsv1.Deployment) error {
						apiServerDeployment.Status.ObservedGeneration = observedGeneration
						*actual = apiServerDeployment
						return nil
					})

				err := kubeAPIServer.Wait(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("retry failed with max attempts reached, last error: kube-apiserver not observed at latest generation (%d/%d), logs of newest pod:\n%s", observedGeneration, apiServerDeployment.ObjectMeta.Generation, logs)))
			})

			It("should return error - kube-apiserver does not have enough updated replicas", func() {
				replicas := int32(2)
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, actual *appsv1.Deployment) error {
						apiServerDeployment.Status.UpdatedReplicas = replicas
						*actual = apiServerDeployment
						return nil
					})

				err := kubeAPIServer.Wait(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("retry failed with max attempts reached, last error: kube-apiserver does not have enough updated replicas (%d/%d), logs of newest pod:\n%s", replicas, apiServerDeployment.Spec.Replicas, logs)))
			})

			It("should return error - kube-apiserver deployment has outdated replicas", func() {
				replicas := int32(2)
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, actual *appsv1.Deployment) error {
						apiServerDeployment.Status.Replicas = replicas
						*actual = apiServerDeployment
						return nil
					})

				err := kubeAPIServer.Wait(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("retry failed with max attempts reached, last error: kube-apiserver deployment has outdated replicas, logs of newest pod:\n%s", logs)))
			})

			It("should return error - kube-apiserver deployment has outdated replicas", func() {
				replicas := int32(2)
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, actual *appsv1.Deployment) error {
						apiServerDeployment.Status.AvailableReplicas = replicas
						*actual = apiServerDeployment
						return nil
					})

				err := kubeAPIServer.Wait(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("retry failed with max attempts reached, last error: kube-apiserver does not have enough available replicas (%d/%d), logs of newest pod:\n%s", replicas, apiServerDeployment.Spec.Replicas, logs)))
			})

		})
	})

	Describe("#WaitCleanup", func() {
		BeforeEach(func() {
			kubeAPIServer, _ = NewAPIServerBuilder().Build()
		})

		It("should be successful", func() {
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

			err := kubeAPIServer.WaitCleanup(ctx)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error - deployment still exists", func() {
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{}))

			err := kubeAPIServer.WaitCleanup(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error - fails to retrieve deployment", func() {
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeError)

			err := kubeAPIServer.WaitCleanup(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(fakeError))
		})
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			kubeAPIServer, valuesProvider = NewAPIServerBuilder().Build()
		})

		// set insufficient secrets and then validate
		Context("missing secret information", func() {
			It("should return an error - missing CA secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing CA secret information")))
			})

			It("should return an error - missing CA front-proxy secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA: component.Secret{Name: "ca", Checksum: checksumCA},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing CA front-proxy secret information")))
			})

			It("should return an error - missing TLS Server secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA:           component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy: component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing TLS server secret information")))
			})

			It("should return an error - missing kube-aggregator secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA:           component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy: component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:    component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing kube aggregator secret information")))
			})

			It("should return an error - missing kubelet secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA:             component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:   component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:      component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator: component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing kubelet secret information")))
			})

			It("should return an error - missing staticToken secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing staticToken secret information")))
			})

			It("should return an error - missing service account key secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:          component.Secret{Name: "static-token", Checksum: checksumStaticToken},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing service account key secret information")))
			})

			It("should return an error - missing etcd CA secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:          component.Secret{Name: "static-token", Checksum: checksumStaticToken},
					ServiceAccountKey:    component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing etcd CA secret information")))
			})

			It("should return an error - missing etcd client TLS secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:          component.Secret{Name: "static-token", Checksum: checksumStaticToken},
					ServiceAccountKey:    component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
					EtcdCA:               component.Secret{Name: "ca-etcd", Checksum: checksumCAEtcd},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing etcd client TLS secret information")))
			})

			It("should return an error - missing basic auth secret information", func() {
				apiserver, _ := NewAPIServerBuilder().
					SetBasicAuthenticationEnabled(true).
					Build()

				apiserver.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:          component.Secret{Name: "static-token", Checksum: checksumStaticToken},
					ServiceAccountKey:    component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
					EtcdCA:               component.Secret{Name: "ca-etcd", Checksum: checksumCAEtcd},
					EtcdClientTLS:        component.Secret{Name: "etcd-client-tls", Checksum: checksumETCDClientTLS},
				})
				err := apiserver.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing basic auth secret information")))
			})

			It("should return an error - missing konnectivity server certificate secret information", func() {
				apiserver, _ := NewAPIServerBuilder().
					SetKonnectivityTunnelEnabled(true).
					Build()

				apiserver.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:          component.Secret{Name: "static-token", Checksum: checksumStaticToken},
					ServiceAccountKey:    component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
					EtcdCA:               component.Secret{Name: "ca-etcd", Checksum: checksumCAEtcd},
					EtcdClientTLS:        component.Secret{Name: "etcd-client-tls", Checksum: checksumETCDClientTLS},
				})
				err := apiserver.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing konnectivity server certificate secret information")))
			})

			It("should return an error - missing konnectivity server kubeconfig secret information", func() {
				apiserver, _ := NewAPIServerBuilder().
					SetKonnectivityTunnelEnabled(true).
					Build()

				apiserver.SetSecrets(Secrets{
					CA:                      component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:            component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:               component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:          component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet:    component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:             component.Secret{Name: "static-token", Checksum: checksumStaticToken},
					ServiceAccountKey:       component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
					EtcdCA:                  component.Secret{Name: "ca-etcd", Checksum: checksumCAEtcd},
					EtcdClientTLS:           component.Secret{Name: "etcd-client-tls", Checksum: checksumETCDClientTLS},
					KonnectivityServerCerts: component.Secret{Name: "konnectivity-server", Checksum: checksumKonnectivityServer},
				})
				err := apiserver.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing konnectivity server kubeconfig secret information")))
			})

			It("should return an error - missing konnectivity server kubeconfig secret information", func() {
				apiserver, _ := NewAPIServerBuilder().
					SetKonnectivityTunnelEnabled(true).
					SetSNIValues(&APIServerSNIValues{SNIEnabled: true}).
					Build()

				apiserver.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:          component.Secret{Name: "static-token", Checksum: checksumStaticToken},
					ServiceAccountKey:    component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
					EtcdCA:               component.Secret{Name: "ca-etcd", Checksum: checksumCAEtcd},
					EtcdClientTLS:        component.Secret{Name: "etcd-client-tls", Checksum: checksumETCDClientTLS},
				})
				err := apiserver.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing konnectivity server client certificate secret information")))
			})

			It("should return an error - missing vpn seed secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:          component.Secret{Name: "static-token", Checksum: checksumStaticToken},
					ServiceAccountKey:    component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
					EtcdCA:               component.Secret{Name: "ca-etcd", Checksum: checksumCAEtcd},
					EtcdClientTLS:        component.Secret{Name: "etcd-client-tls", Checksum: checksumETCDClientTLS},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing vpn seed secret information")))
			})

			It("should return an error - missing vpn seed  TLS auth secret information", func() {
				kubeAPIServer.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:          component.Secret{Name: "static-token", Checksum: checksumStaticToken},
					ServiceAccountKey:    component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
					EtcdCA:               component.Secret{Name: "ca-etcd", Checksum: checksumCAEtcd},
					EtcdClientTLS:        component.Secret{Name: "etcd-client-tls", Checksum: checksumETCDClientTLS},
					VpnSeed:              component.Secret{Name: "vpn-seed", Checksum: checksumVPNSeed},
				})
				err := kubeAPIServer.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing vpn seed  TLS auth secret information")))
			})
			It("should return an error - missing etcd encryption secret information", func() {
				apiserver, _ := NewAPIServerBuilder().
					SetEtcdEncryptionEnabled(true).
					Build()

				apiserver.SetSecrets(Secrets{
					CA:                   component.Secret{Name: "ca", Checksum: checksumCA},
					CAFrontProxy:         component.Secret{Name: "ca-front-proxy", Checksum: checksumCaFrontProxy},
					TLSServer:            component.Secret{Name: "kube-apiserver", Checksum: checksumTLSServer},
					KubeAggregator:       component.Secret{Name: "kube-aggregator", Checksum: checksumKubeAggregator},
					KubeAPIServerKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: checksumKubeAPIServerKubelet},
					StaticToken:          component.Secret{Name: "static-token", Checksum: checksumStaticToken},
					ServiceAccountKey:    component.Secret{Name: "service-account-key", Checksum: checksumServiceAccountKey},
					EtcdCA:               component.Secret{Name: "ca-etcd", Checksum: checksumCAEtcd},
					EtcdClientTLS:        component.Secret{Name: "etcd-client-tls", Checksum: checksumETCDClientTLS},
					VpnSeed:              component.Secret{Name: "vpn-seed", Checksum: checksumVPNSeed},
					VpnSeedTLSAuth:       component.Secret{Name: "vpn-seed-tlsauth", Checksum: checksumVPNSeedTLSAuth},
				})
				err := apiserver.Deploy(ctx)
				Expect(err).To(MatchError(ContainSubstring("missing etcd encryption secret information")))
			})

			Context("Validate other configuration set at deploy time", func() {
				It("should return an error - the ingress of the loadbalancer (Shoot APIServer svc or SNI LB) has to be set", func() {
					kubeAPIServer.SetShootOutOfClusterAPIServerAddress("")
					err := kubeAPIServer.Deploy(ctx)
					Expect(err).To(MatchError(ContainSubstring("the ingress of the SNI loadbalancer has to be set to calculate the API Server health check token and to confgiure the SNI pod mutator")))
				})

				It("should return an error - the health check token has to be set", func() {
					kubeAPIServer.SetHealthCheckToken("")
					err := kubeAPIServer.Deploy(ctx)
					Expect(err).To(MatchError(ContainSubstring("the API Server health check token has to be set")))
				})

				It("should return an error - the cluster IP of the service of the apiserver has to be set when SNI is enabled", func() {
					sni := APIServerSNIValues{
						SNIEnabled:           true,
						SNIPodMutatorEnabled: false,
					}

					apiserver, _ := NewAPIServerBuilder().
						SetSNIValues(&sni).
						Build()

					apiserver.SetShootAPIServerClusterIP("")

					err := apiserver.Deploy(ctx)
					Expect(err).To(MatchError(ContainSubstring("the cluster IP of the service of the apiserver has to be set when SNI is enabled")))
				})
			})
		})

		Context("Deployment with valid configuration", func() {
			It("should deploy components successfully", func() {
				Expect(expectWithValueProvider(ctx, kubeAPIServer, valuesProvider)).ToNot(HaveOccurred())
			})

			Context("Deploy with Admission Plugins", func() {
				var (
					pluginNamePodNodeSelector   = "PodNodeSelector"
					pluginConfigPodNodeSelector = `podNodeSelectorPluginConfig:
  clusterDefaultNodeSelector: scheduler.alpha.kubernetes.io/node-selector=mynodelabel`

					pluginNamePriority   = "Priority"
					pluginConfigPriority = `priorityPluginConfig:
  priorityClassName: gardener`
				)

				DescribeTable("Tests with custom Admission Plugin", func(pluginName, pluginConfig string) {
					config := &gardencorev1beta1.KubeAPIServerConfig{
						AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{
							{
								Name: pluginName,
								Config: &runtime.RawExtension{
									Raw: []byte(pluginConfig),
								},
							},
						},
					}

					apiserver, vp := NewAPIServerBuilder().
						SetAPIServerConfig(config).
						Build()

					Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())

				},
					Entry("should deploy successfully with custom admission plugin", pluginNamePodNodeSelector, pluginConfigPodNodeSelector),
					Entry("should deploy successfully - admission plugin overwrites default plugin", pluginNamePriority, pluginConfigPriority),
				)
			})

			Context("Deploy with OIDCConfig", func() {
				It("should deploy successfully", func() {
					config := &gardencorev1beta1.KubeAPIServerConfig{
						OIDCConfig: &gardencorev1beta1.OIDCConfig{
							CABundle:     pointer.StringPtr("my-ca-content"),
							IssuerURL:    pointer.StringPtr("my-url"),
							ClientID:     pointer.StringPtr("my-client-id"),
							GroupsClaim:  pointer.StringPtr("claim"),
							GroupsPrefix: pointer.StringPtr("prefix"),
							RequiredClaims: map[string]string{
								"the": "fix",
							},
							SigningAlgs:    []string{"secure123"},
							UsernameClaim:  pointer.StringPtr("lenox"),
							UsernamePrefix: pointer.StringPtr("garden"),
						},
					}

					apiserver, vp := NewAPIServerBuilder().
						SetAPIServerConfig(config).
						Build()

					Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
				})
			})

			It("Should deploy successfully - Konnectivity server enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetKonnectivityTunnelEnabled(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Konnectivity server enabled, K8s > 1.20", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetKonnectivityTunnelEnabled(true).
					SetShootKubernetesVersion("1.19.0").
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - SNI enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetSNIValues(&APIServerSNIValues{SNIEnabled: true}).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - SNIPodMutator enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetSNIValues(&APIServerSNIValues{SNIEnabled: true, SNIPodMutatorEnabled: true}).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - SNI and Konnectivity server enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetSNIValues(&APIServerSNIValues{SNIEnabled: true}).
					SetKonnectivityTunnelEnabled(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			Context("Deploy with Audit Policy", func() {
				gardenAuditPolicyConfigMapName := "MyAuditPolicy"
				apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
					AuditConfig: &gardencorev1beta1.AuditConfig{
						AuditPolicy: &gardencorev1beta1.AuditPolicy{
							ConfigMapRef: &corev1.ObjectReference{
								Name: gardenAuditPolicyConfigMapName,
							},
						},
					},
				}

				BeforeEach(func() {
					kubeAPIServer, valuesProvider = NewAPIServerBuilder().
						SetAPIServerConfig(apiServerConfig).
						Build()
				})

				It("should deploy components successfully", func() {
					Expect(expectWithValueProvider(ctx, kubeAPIServer, valuesProvider)).ToNot(HaveOccurred())
				})

				It("should return an error - audit policy config map cannot be retrieved from the Garden cluster", func() {
					expectDefaultAdmissionConfigMap(ctx)
					mockGardenClient.EXPECT().Get(ctx, kutil.Key(defaultGardenNamespace, apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(fakeError)

					err := kubeAPIServer.Deploy(ctx)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("deploy the Kube API server audit policy config map to the Seed failed. Retrieval of the audit policy from the ConfigMap '%s' in the Garden cluster failed: fake error", gardenAuditPolicyConfigMapName)))
				})
			})

			It("Should deploy successfully - hibernation enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetHibernationEnabled(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - hibernation enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetBasicAuthenticationEnabled(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - hvpa enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetHvpaEnabled(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - hvpa enabled and Seed K8s version < 1.12", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetHvpaEnabled(true).
					SetSeedKubernetesVersion("1.11.0").
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - hvpa and hibernation enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetHvpaEnabled(true).
					SetHibernationEnabled(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - hvpa and maintenance time window set", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetHvpaEnabled(true).
					SetMaintenanceWindow(&gardencorev1beta1.MaintenanceTimeWindow{
						Begin: "123456+0000",
						End:   "162543+0000",
					}).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - hvpa and shooted Seed set", func() {
				managedSeedAPIServer := &gardencorev1beta1helper.ShootedSeedAPIServer{
					Replicas: pointer.Int32Ptr(2),
				}

				apiserver, vp := NewAPIServerBuilder().
					SetHvpaEnabled(true).
					SetManagedSeedAPIServer(managedSeedAPIServer).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - hvpa and SNI Pod mutator enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetHvpaEnabled(true).
					SetSNIValues(&APIServerSNIValues{SNIEnabled: true, SNIPodMutatorEnabled: true}).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - mountHostCADirectories enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetMountHostCADirectories(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Shoot has deletion timestamp set", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetShootHasDeletionTimestamp(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Shoot has etcd encryption set", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetEtcdEncryptionEnabled(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Shoot is a Shooted Seed with replicas set", func() {
				managedSeedAPIServer := &gardencorev1beta1helper.ShootedSeedAPIServer{
					Replicas: pointer.Int32Ptr(2),
				}

				apiserver, vp := NewAPIServerBuilder().
					SetManagedSeedAPIServer(managedSeedAPIServer).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Shoot deployment already exists", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetDeploymentReplicas(2).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Shoot deployment already exists and HVPA enabled", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetDeploymentReplicas(2).
					SetHvpaEnabled(true).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Shoot is a Shooted Seed with autoscaler set", func() {
				managedSeedAPIServer := &gardencorev1beta1helper.ShootedSeedAPIServer{
					Autoscaler: &gardencorev1beta1helper.ShootedSeedAPIServerAutoscaler{
						MinReplicas: pointer.Int32Ptr(2),
						MaxReplicas: 3,
					},
				}

				apiserver, vp := NewAPIServerBuilder().
					SetManagedSeedAPIServer(managedSeedAPIServer).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			DescribeTable("Tests with a scaling class by annotation", func(scalingClass string) {
				apiserver, vp := NewAPIServerBuilder().
					SetShootAnnotations(map[string]string{
						"alpha.kube-apiserver.scaling.shoot.gardener.cloud/class": scalingClass,
					}).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			},
				Entry("scaling class: small", "small"),
				Entry("scaling class: medium", "medium"),
				Entry("scaling class: large", "large"),
				Entry("scaling class: xlarge", "xlarge"),
				Entry("scaling class: 2xlarge", "2xlarge"),
			)

			DescribeTable("Tests with a scaling class by node count", func(count int) {
				apiserver, vp := NewAPIServerBuilder().
					SetMinimumNodeCount(int32(count)).
					SetMaximumNodeCount(int32(count)).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			},
				Entry("scaling class: small", 2),
				Entry("scaling class: medium", 10),
				Entry("scaling class: large", 50),
				Entry("scaling class: xlarge", 100),
				Entry("scaling class: 2xlarge", 101),
			)

			It("Should deploy successfully - maintenance time window is set", func() {
				apiserver, vp := NewAPIServerBuilder().
					SetMaintenanceWindow(&gardencorev1beta1.MaintenanceTimeWindow{
						Begin: "123456+0000",
						End:   "162543+0000",
					}).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Service account signing key and issuer are set", func() {
				apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
					ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
						Issuer: pointer.StringPtr("my-issuer"),
						SigningKeySecret: &corev1.LocalObjectReference{
							Name: "signingKeySecretName",
						},
					},
				}

				apiserver, vp := NewAPIServerBuilder().
					SetAPIServerConfig(apiServerConfig).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Deploy when node network is set", func() {
				_, ipNet, _ := net.ParseCIDR("8.8.8.8/32")
				apiserver, vp := NewAPIServerBuilder().
					SetNodeNetwork(ipNet).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Watch Cache Size configured", func() {
				apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
					WatchCacheSizes: &gardencorev1beta1.WatchCacheSizes{
						Default: pointer.Int32Ptr(5),
						Resources: []gardencorev1beta1.ResourceWatchCacheSize{
							{
								APIGroup:  pointer.StringPtr("super-nice-apigroup"),
								Resource:  "resource",
								CacheSize: 10,
							},
							{
								APIGroup:  pointer.StringPtr("super-nice-apigroup-v2"),
								Resource:  "resource-2",
								CacheSize: 11,
							},
						},
					},
				}

				apiserver, vp := NewAPIServerBuilder().
					SetAPIServerConfig(apiServerConfig).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Feature Gates configured", func() {
				apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
					KubernetesConfig: gardencorev1beta1.KubernetesConfig{FeatureGates: map[string]bool{
						"feature-one": true,
						"feature-two": false,
					}},
				}

				apiserver, vp := NewAPIServerBuilder().
					SetAPIServerConfig(apiServerConfig).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Mutating Requests configured", func() {
				apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
					Requests: &gardencorev1beta1.KubeAPIServerRequests{
						MaxNonMutatingInflight: pointer.Int32Ptr(10),
						MaxMutatingInflight:    pointer.Int32Ptr(11),
					},
				}

				apiserver, vp := NewAPIServerBuilder().
					SetAPIServerConfig(apiServerConfig).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - Runtime Config set", func() {
				apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
					RuntimeConfig: map[string]bool{
						"autoscaling/v2alpha1": true,
						"autoscaling/v2alpha2": false,
					},
				}

				apiserver, vp := NewAPIServerBuilder().
					SetAPIServerConfig(apiServerConfig).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			It("Should deploy successfully - API Audiences set", func() {
				apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
					APIAudiences: []string{
						"kubernetes",
						"https://myserver.example.com",
					},
				}

				apiserver, vp := NewAPIServerBuilder().
					SetAPIServerConfig(apiServerConfig).
					Build()

				Expect(expectWithValueProvider(ctx, apiserver, vp)).ToNot(HaveOccurred())
			})

			Context("Tests for various Kubernetes versions", func() {
				DescribeTable("Tests for various Kubernetes versions", func(shootK8sVersion string) {
					kubeAPIServer, valuesProvider = NewAPIServerBuilder().
						SetShootKubernetesVersion(shootK8sVersion).
						Build()

					shaAdmissionConfigMap := expectDefaultAdmissionConfigMap(ctx)
					shaAuditConfigMap := expectAuditPolicyConfigMap(ctx, nil, shootK8sVersion)
					expectNetworkPolicies(ctx, valuesProvider)
					expectPodDisruptionBudget(ctx)
					deploymentReplicas := expectDeployment(ctx,
						valuesProvider,
						shaAuditConfigMap,
						shaAdmissionConfigMap,
						nil,
						nil,
						nil,
					)

					// default is vpa & hpa, so hvpa is deleted
					expectAutoscaler(ctx, valuesProvider, deploymentReplicas)
					err := kubeAPIServer.Deploy(ctx)
					Expect(err).ToNot(HaveOccurred())
				},
					Entry("1.10", "1.10"),
					Entry("1.11", "1.11"),
					Entry("1.12", "1.12"),
					Entry("1.13", "1.13"),
					Entry("1.14", "1.14"),
					Entry("1.15", "1.15"),
					Entry("1.16", "1.16"),
					Entry("1.17", "1.17"),
					Entry("1.18", "1.18"),
					Entry("1.19", "1.19"),
				)
			})

		})
	})
})

func expectWithValueProvider(ctx context.Context, apiserver KubeAPIServer, valuesProvider KubeAPIServerValuesProvider) error {
	shaOIDCCASecret := expectOIDCCABundle(ctx, valuesProvider)
	shaServiceAccountSigningKey := expectServiceAccountSigningKey(ctx, valuesProvider)
	shaConfigMapEgressSelection := expectEgressSelectorConfigMap(ctx, valuesProvider)

	expectServiceAccount(ctx, valuesProvider)
	expectRBAC(ctx, valuesProvider)

	shaAdmissionConfigMap := expectAdmissionConfigMap(ctx, valuesProvider)
	shaAuditConfigMap := expectAuditPolicyConfigMap(ctx, valuesProvider.GetAPIServerConfig(), valuesProvider.GetShootKubernetesVersion())

	expectNetworkPolicies(ctx, valuesProvider)
	expectPodDisruptionBudget(ctx)

	deploymentReplicas := expectDeployment(ctx,
		valuesProvider,
		shaAuditConfigMap,
		shaAdmissionConfigMap,
		shaOIDCCASecret,
		shaConfigMapEgressSelection,
		shaServiceAccountSigningKey,
	)

	expectAutoscaler(ctx, valuesProvider, deploymentReplicas)
	return apiserver.Deploy(ctx)
}

func expectDeployment(ctx context.Context, valuesProvider KubeAPIServerValuesProvider, shaAuditConfigMap, shaAdmissionConfigMap string, shaOIDCCASecret, shaConfigMapEgressSelection, shaeServiceAccountSigningKeySecret *string) *int32 {
	existingDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver",
			Namespace: defaultSeedNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: valuesProvider.GetDeploymentReplicas(),
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
				{
					Name: "kube-apiserver",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
					},
				},
			},
			},
			}},
	}

	admissionPlugins := getAdmissionPlugins(valuesProvider)

	command := computeKubeAPIServerCommand(valuesProvider, admissionPlugins)

	expectedDeployment, deploymentReplicas := getExpectedDeplomentFor(
		valuesProvider,
		command,
		existingDeployment,
		shaeServiceAccountSigningKeySecret,
		shaConfigMapEgressSelection,
		shaOIDCCASecret,
		// both audit & admission config maps are always deployed
		&shaAuditConfigMap,
		&shaAdmissionConfigMap,
	)

	// depending on whether the deployment already exists, the expected deployment differs
	if valuesProvider.DeploymentAlreadyExists() {
		mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
			func(_ context.Context, _ client.ObjectKey, actual *appsv1.Deployment) error {
				*actual = existingDeployment
				return nil
			}).Times(2)
		mockSeedClient.EXPECT().Update(ctx, expectedDeployment)
	} else {
		mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).
			Return(apierrors.NewNotFound(schema.GroupResource{}, "foo")).Times(2)
		mockSeedClient.EXPECT().Create(ctx, expectedDeployment)
	}

	return deploymentReplicas
}

func expectNetworkPolicies(ctx context.Context, valuesProvider KubeAPIServerValuesProvider) {
	expectDefaultNetpolAllowFromShootAPIServer(ctx)
	expectNetpolAllowKubeAPIServer(ctx, valuesProvider)
	expectDefaultNetpolAllowToShootAPIServer(ctx)
}

func getAdmissionPlugins(valuesProvider KubeAPIServerValuesProvider) []gardencorev1beta1.AdmissionPlugin {
	admissionPlugins := kubernetes.GetAdmissionPluginsForVersion(valuesProvider.GetShootKubernetesVersion())
	if valuesProvider.GetAPIServerConfig() != nil {
		for _, plugin := range valuesProvider.GetAPIServerConfig().AdmissionPlugins {
			pluginOverwritesDefault := false

			for i, defaultPlugin := range admissionPlugins {
				if defaultPlugin.Name == plugin.Name {
					pluginOverwritesDefault = true
					admissionPlugins[i] = plugin
					break
				}
			}

			if !pluginOverwritesDefault {
				admissionPlugins = append(admissionPlugins, plugin)
			}
		}
	}
	return admissionPlugins
}
