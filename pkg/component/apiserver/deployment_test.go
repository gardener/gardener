// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

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
				secretCAETCD     = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd"}}
				secretETCDClient = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "etcd-client"}}
				secretServer     = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "server"}}
				values           = Values{
					FeatureGates: map[string]bool{"Foo": true, "Bar": false},
					Requests: &gardencorev1beta1.APIServerRequests{
						MaxMutatingInflight:    ptr.To[int32](1),
						MaxNonMutatingInflight: ptr.To[int32](2),
					},
					Logging: &gardencorev1beta1.APIServerLogging{
						Verbosity:           ptr.To[int32](3),
						HTTPAccessVerbosity: ptr.To[int32](4),
					},
					WatchCacheSizes: &gardencorev1beta1.WatchCacheSizes{
						Default: ptr.To[int32](6),
						Resources: []gardencorev1beta1.ResourceWatchCacheSize{
							{APIGroup: ptr.To("foo"), Resource: "bar"},
							{APIGroup: ptr.To("baz"), Resource: "foo", CacheSize: 7},
						},
					},
				}
			)

			InjectDefaultSettings(deployment, namePrefix, values, secretCAETCD, secretETCDClient, secretServer)

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
											SecretName:  secretETCDClient.Name,
											DefaultMode: ptr.To[int32](0640),
										},
									},
								},
								{
									Name: "server",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName:  secretServer.Name,
											DefaultMode: ptr.To[int32](0640),
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
