// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package apiserver_test

import (
	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/component/apiserver"
)

var _ = Describe("Deployment", func() {
	Describe("#InjectDefaultSettings", func() {
		It("should inject the correct settings", func() {
			deployment := &appsv1.Deployment{}
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{})

			var (
				namePrefix       = "foo"
				k8sVersion       = semver.MustParse("1.24.5")
				secretCAETCD     = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd"}}
				secretETCDClient = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "etcd-client"}}
				secretServer     = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "server"}}
				values           = Values{
					FeatureGates: map[string]bool{"Foo": true, "Bar": false},
					Requests: &gardencorev1beta1.KubeAPIServerRequests{
						MaxMutatingInflight:    pointer.Int32(1),
						MaxNonMutatingInflight: pointer.Int32(2),
					},
					Logging: &gardencorev1beta1.KubeAPIServerLogging{
						Verbosity:           pointer.Int32(3),
						HTTPAccessVerbosity: pointer.Int32(4),
					},
					WatchCacheSizes: &gardencorev1beta1.WatchCacheSizes{
						Default: pointer.Int32(6),
						Resources: []gardencorev1beta1.ResourceWatchCacheSize{
							{APIGroup: pointer.String("foo"), Resource: "bar"},
							{APIGroup: pointer.String("baz"), Resource: "foo", CacheSize: 7},
						},
					},
				}
			)

			InjectDefaultSettings(deployment, namePrefix, values, k8sVersion, secretCAETCD, secretETCDClient, secretServer)

			Expect(deployment).To(Equal(&appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Args: []string{
									"--http2-max-streams-per-connection=1000",
									"--etcd-cafile=/srv/kubernetes/etcd/ca/bundle.crt",
									"--etcd-certfile=/srv/kubernetes/etcd/client/tls.crt",
									"--etcd-keyfile=/srv/kubernetes/etcd/client/tls.key",
									"--etcd-servers=https://" + namePrefix + "etcd-main-client:2379",
									"--livez-grace-period=1m",
									"--profiling=false",
									"--shutdown-delay-duration=15s",
									"--tls-cert-file=/srv/kubernetes/apiserver/tls.crt",
									"--tls-private-key-file=/srv/kubernetes/apiserver/tls.key",
									"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_CHACHA20_POLY1305_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
									"--feature-gates=Bar=false,Foo=true",
									"--max-requests-inflight=2",
									"--max-mutating-requests-inflight=1",
									"--vmodule=httplog=4",
									"--v=3",
									"--default-watch-cache-size=6",
									"--watch-cache-sizes=bar.foo#0,foo.baz#7",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "ca-etcd",
										MountPath: "/srv/kubernetes/etcd/ca",
									},
									{
										Name:      "etcd-client",
										MountPath: "/srv/kubernetes/etcd/client",
									},
									{
										Name:      "server",
										MountPath: "/srv/kubernetes/apiserver",
									},
								},
							}},
							Volumes: []corev1.Volume{
								{
									Name: "ca-etcd",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretCAETCD.Name,
										},
									},
								},
								{
									Name: "etcd-client",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretETCDClient.Name,
										},
									},
								},
								{
									Name: "server",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretServer.Name,
										},
									},
								},
							},
						},
					},
				},
			}))
		})
	})
})
