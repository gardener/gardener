// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package staticpod_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenadm/staticpod"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Translator", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().Build()
	})

	Describe("#Translate", func() {
		When("object is a Deployment", func() {
			var deployment *appsv1.Deployment

			BeforeEach(func() {
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "foo",
						Namespace:   "bar",
						Labels:      map[string]string{"will": "be-ignored"},
						Annotations: map[string]string{"this-as": "well"},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels:      map[string]string{"baz": "foo"},
								Annotations: map[string]string{"bar": "baz"},
							},
							Spec: corev1.PodSpec{
								SecurityContext: &corev1.PodSecurityContext{FSGroup: ptr.To[int64](65534)},
							},
						},
					},
				}
			})

			It("should successfully translate a deployment w/o volumes", func() {
				Expect(Translate(ctx, fakeClient, deployment)).To(HaveExactElements(extensionsv1alpha1.File{
					Path:        "/etc/kubernetes/manifests/" + deployment.Name + ".yaml",
					Permissions: ptr.To[uint32](0640),
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
kind: Pod
metadata:
  annotations:
    bar: baz
  creationTimestamp: null
  labels:
    baz: foo
    static-pod: "true"
  name: foo
  namespace: kube-system
spec:
  containers: null
  hostAliases:
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: 127.0.0.1
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: ::1
  hostNetwork: true
  priorityClassName: system-node-critical
  securityContext:
    fsGroup: 0
status: {}
`))}},
				}))
			})

			It("should fail translate a deployment whose ConfigMap volumes refer to non-existing objects", func() {
				deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "some-configmap"}}}},
				}

				Expect(Translate(ctx, fakeClient, deployment)).Error().To(MatchError(ContainSubstring("failed reading ConfigMap")))
			})

			It("should fail translate a deployment whose Secret volumes refer to non-existing objects", func() {
				deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "some-secret"}}},
				}

				Expect(Translate(ctx, fakeClient, deployment)).Error().To(MatchError(ContainSubstring("failed reading Secret")))
			})

			It("should fail translate a deployment whose projected ConfigMap volumes refer to non-existing objects", func() {
				deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "some-configmap"}}}}}}},
				}

				Expect(Translate(ctx, fakeClient, deployment)).Error().To(MatchError(ContainSubstring("failed reading ConfigMap")))
			})

			It("should fail translate a deployment whose Secret volumes refer to non-existing objects", func() {
				deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{Secret: &corev1.SecretProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "some-secret"}}}}}}},
				}

				Expect(Translate(ctx, fakeClient, deployment)).Error().To(MatchError(ContainSubstring("failed reading Secret")))
			})

			It("should successfully translate a deployment w/ volumes", func() {
				var (
					configMap1 = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: deployment.Namespace},
						Data:       map[string]string{"cm1file1.txt": "some-content", "cm1file2.txt": "more-content"},
					}
					configMap2 = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: deployment.Namespace},
						Data: map[string]string{"cm2file1.txt": "even-more-content", "cm2file2.txt": `apiVersion: v1
clusters:
- cluster:
    server: https://foo.local.gardener.cloud
  name: test
kind: Config
`},
					}
					secret1 = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: deployment.Namespace},
						Data:       map[string][]byte{"secret1file1.txt": []byte("very-secret"), "secret1file2.txt": []byte("super-secret")},
					}
					secret2 = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: deployment.Namespace},
						Data:       map[string][]byte{"secret2file1.txt": []byte("highly-secret"), "secret2file2.txt": []byte("please-dont-detect")},
					}
				)

				Expect(fakeClient.Create(ctx, configMap1)).To(Succeed())
				Expect(fakeClient.Create(ctx, configMap2)).To(Succeed())
				Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
				Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

				deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "v1", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
					{Name: "v2", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "secret1"}}},
					{Name: "v3", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "cm2"}}},
							{Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{Name: "secret2"},
								Items:                []corev1.KeyToPath{{Key: "secret2file2.txt", Path: "mystery.txt"}},
							}},
						},
						DefaultMode: ptr.To[int32](0666),
					}}},
				}

				Expect(Translate(ctx, fakeClient, deployment)).To(ConsistOf(
					extensionsv1alpha1.File{
						Path:        "/etc/kubernetes/manifests/" + deployment.Name + ".yaml",
						Permissions: ptr.To[uint32](0640),
						Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
kind: Pod
metadata:
  annotations:
    bar: baz
  creationTimestamp: null
  labels:
    baz: foo
    static-pod: "true"
  name: foo
  namespace: kube-system
spec:
  containers: null
  hostAliases:
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: 127.0.0.1
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: ::1
  hostNetwork: true
  priorityClassName: system-node-critical
  securityContext:
    fsGroup: 0
  volumes:
  - hostPath:
      path: /var/lib/foo/v1
    name: v1
  - hostPath:
      path: /var/lib/foo/v2
    name: v2
  - hostPath:
      path: /var/lib/foo/v3
    name: v3
status: {}
`))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + deployment.Name + "/" + deployment.Spec.Template.Spec.Volumes[0].Name + "/cm1file1.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(configMap1.Data["cm1file1.txt"]))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + deployment.Name + "/" + deployment.Spec.Template.Spec.Volumes[0].Name + "/cm1file2.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(configMap1.Data["cm1file2.txt"]))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + deployment.Name + "/" + deployment.Spec.Template.Spec.Volumes[1].Name + "/secret1file1.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(secret1.Data["secret1file1.txt"])}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + deployment.Name + "/" + deployment.Spec.Template.Spec.Volumes[1].Name + "/secret1file2.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(secret1.Data["secret1file2.txt"])}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + deployment.Name + "/" + deployment.Spec.Template.Spec.Volumes[2].Name + "/cm2file1.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(configMap2.Data["cm2file1.txt"]))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + deployment.Name + "/" + deployment.Spec.Template.Spec.Volumes[2].Name + "/cm2file2.txt",
						Permissions: ptr.To[uint32](0640),
						Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
clusters:
- cluster:
    server: https://foo.local.gardener.cloud
  name: test
kind: Config
`))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + deployment.Name + "/" + deployment.Spec.Template.Spec.Volumes[2].Name + "/mystery.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(secret2.Data["secret2file2.txt"])}},
					},
				))
			})
		})

		When("object is a StatefulSet", func() {
			var statefulSet *appsv1.StatefulSet

			BeforeEach(func() {
				statefulSet = &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "foo",
						Namespace:   "bar",
						Labels:      map[string]string{"will": "be-ignored"},
						Annotations: map[string]string{"this-as": "well"},
					},
					Spec: appsv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels:      map[string]string{"baz": "foo"},
								Annotations: map[string]string{"bar": "baz"},
							},
							Spec: corev1.PodSpec{
								SecurityContext: &corev1.PodSecurityContext{FSGroup: ptr.To[int64](65534)},
							},
						},
					},
				}
			})

			It("should successfully translate a statefulSet w/o volumes", func() {
				Expect(Translate(ctx, fakeClient, statefulSet)).To(HaveExactElements(extensionsv1alpha1.File{
					Path:        "/etc/kubernetes/manifests/" + statefulSet.Name + ".yaml",
					Permissions: ptr.To[uint32](0640),
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
kind: Pod
metadata:
  annotations:
    bar: baz
  creationTimestamp: null
  labels:
    baz: foo
    static-pod: "true"
  name: foo
  namespace: kube-system
spec:
  containers: null
  hostAliases:
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: 127.0.0.1
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: ::1
  hostNetwork: true
  priorityClassName: system-node-critical
  securityContext:
    fsGroup: 0
status: {}
`))}},
				}))
			})

			It("should fail translate a statefulSet whose ConfigMap volumes refer to non-existing objects", func() {
				statefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "some-configmap"}}}},
				}

				Expect(Translate(ctx, fakeClient, statefulSet)).Error().To(MatchError(ContainSubstring("failed reading ConfigMap")))
			})

			It("should fail translate a statefulSet whose Secret volumes refer to non-existing objects", func() {
				statefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "some-secret"}}},
				}

				Expect(Translate(ctx, fakeClient, statefulSet)).Error().To(MatchError(ContainSubstring("failed reading Secret")))
			})

			It("should fail translate a statefulSet whose projected ConfigMap volumes refer to non-existing objects", func() {
				statefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "some-configmap"}}}}}}},
				}

				Expect(Translate(ctx, fakeClient, statefulSet)).Error().To(MatchError(ContainSubstring("failed reading ConfigMap")))
			})

			It("should fail translate a statefulSet whose Secret volumes refer to non-existing objects", func() {
				statefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{Secret: &corev1.SecretProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "some-secret"}}}}}}},
				}

				Expect(Translate(ctx, fakeClient, statefulSet)).Error().To(MatchError(ContainSubstring("failed reading Secret")))
			})

			It("should successfully translate a statefulSet w/ volumes", func() {
				var (
					configMap1 = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: statefulSet.Namespace},
						Data:       map[string]string{"cm1file1.txt": "some-content", "cm1file2.txt": "more-content"},
					}
					configMap2 = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: statefulSet.Namespace},
						Data: map[string]string{"cm2file1.txt": "even-more-content", "cm2file2.txt": `apiVersion: v1
clusters:
- cluster:
    server: https://foo.local.gardener.cloud
  name: test
kind: Config
`},
					}
					secret1 = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: statefulSet.Namespace},
						Data:       map[string][]byte{"secret1file1.txt": []byte("very-secret"), "secret1file2.txt": []byte("super-secret")},
					}
					secret2 = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: statefulSet.Namespace},
						Data:       map[string][]byte{"secret2file1.txt": []byte("highly-secret"), "secret2file2.txt": []byte("please-dont-detect")},
					}
				)

				Expect(fakeClient.Create(ctx, configMap1)).To(Succeed())
				Expect(fakeClient.Create(ctx, configMap2)).To(Succeed())
				Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
				Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

				statefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
					{Name: "v1", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
					{Name: "v2", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "secret1"}}},
					{Name: "v3", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "cm2"}}},
							{Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{Name: "secret2"},
								Items:                []corev1.KeyToPath{{Key: "secret2file2.txt", Path: "mystery.txt"}},
							}},
						},
						DefaultMode: ptr.To[int32](0666),
					}}},
				}

				Expect(Translate(ctx, fakeClient, statefulSet)).To(ConsistOf(
					extensionsv1alpha1.File{
						Path:        "/etc/kubernetes/manifests/" + statefulSet.Name + ".yaml",
						Permissions: ptr.To[uint32](0640),
						Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
kind: Pod
metadata:
  annotations:
    bar: baz
  creationTimestamp: null
  labels:
    baz: foo
    static-pod: "true"
  name: foo
  namespace: kube-system
spec:
  containers: null
  hostAliases:
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: 127.0.0.1
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: ::1
  hostNetwork: true
  priorityClassName: system-node-critical
  securityContext:
    fsGroup: 0
  volumes:
  - hostPath:
      path: /var/lib/foo/v1
    name: v1
  - hostPath:
      path: /var/lib/foo/v2
    name: v2
  - hostPath:
      path: /var/lib/foo/v3
    name: v3
status: {}
`))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + statefulSet.Name + "/" + statefulSet.Spec.Template.Spec.Volumes[0].Name + "/cm1file1.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(configMap1.Data["cm1file1.txt"]))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + statefulSet.Name + "/" + statefulSet.Spec.Template.Spec.Volumes[0].Name + "/cm1file2.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(configMap1.Data["cm1file2.txt"]))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + statefulSet.Name + "/" + statefulSet.Spec.Template.Spec.Volumes[1].Name + "/secret1file1.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(secret1.Data["secret1file1.txt"])}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + statefulSet.Name + "/" + statefulSet.Spec.Template.Spec.Volumes[1].Name + "/secret1file2.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(secret1.Data["secret1file2.txt"])}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + statefulSet.Name + "/" + statefulSet.Spec.Template.Spec.Volumes[2].Name + "/cm2file1.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(configMap2.Data["cm2file1.txt"]))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + statefulSet.Name + "/" + statefulSet.Spec.Template.Spec.Volumes[2].Name + "/cm2file2.txt",
						Permissions: ptr.To[uint32](0640),
						Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
clusters:
- cluster:
    server: https://foo.local.gardener.cloud
  name: test
kind: Config
`))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + statefulSet.Name + "/" + statefulSet.Spec.Template.Spec.Volumes[2].Name + "/mystery.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(secret2.Data["secret2file2.txt"])}},
					},
				))
			})
		})

		When("object is a Pod", func() {
			var pod *corev1.Pod

			BeforeEach(func() {
				pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels:          map[string]string{"baz": "foo"},
						Annotations:     map[string]string{"bar": "baz"},
						Name:            "foo",
						ResourceVersion: "1",
						Finalizers:      []string{"bar"},
						OwnerReferences: []metav1.OwnerReference{{}},
					},
				}
			})

			It("should successfully translate a pod w/o volumes", func() {
				Expect(Translate(ctx, fakeClient, pod)).To(HaveExactElements(extensionsv1alpha1.File{
					Path:        "/etc/kubernetes/manifests/" + pod.Name + ".yaml",
					Permissions: ptr.To[uint32](0640),
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
kind: Pod
metadata:
  annotations:
    bar: baz
  creationTimestamp: null
  labels:
    baz: foo
    static-pod: "true"
  name: foo
  namespace: kube-system
spec:
  containers: null
  hostAliases:
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: 127.0.0.1
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: ::1
  hostNetwork: true
  priorityClassName: system-node-critical
status: {}
`))}},
				}))
			})

			It("should fail translate a pod whose ConfigMap volumes refer to non-existing objects", func() {
				pod.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "some-configmap"}}}},
				}

				Expect(Translate(ctx, fakeClient, pod)).Error().To(MatchError(ContainSubstring("failed reading ConfigMap")))
			})

			It("should fail translate a pod whose Secret volumes refer to non-existing objects", func() {
				pod.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "some-secret"}}},
				}

				Expect(Translate(ctx, fakeClient, pod)).Error().To(MatchError(ContainSubstring("failed reading Secret")))
			})

			It("should fail translate a pod whose projected ConfigMap volumes refer to non-existing objects", func() {
				pod.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "some-configmap"}}}}}}},
				}

				Expect(Translate(ctx, fakeClient, pod)).Error().To(MatchError(ContainSubstring("failed reading ConfigMap")))
			})

			It("should fail translate a pod whose Secret volumes refer to non-existing objects", func() {
				pod.Spec.Volumes = []corev1.Volume{
					{Name: "foo", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{Secret: &corev1.SecretProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "some-secret"}}}}}}},
				}

				Expect(Translate(ctx, fakeClient, pod)).Error().To(MatchError(ContainSubstring("failed reading Secret")))
			})

			It("should successfully translate a pod w/ volumes", func() {
				var (
					configMap1 = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: pod.Namespace},
						Data:       map[string]string{"cm1file1.txt": "some-content", "cm1file2.txt": "more-content"},
					}
					configMap2 = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: pod.Namespace},
						Data: map[string]string{"cm2file1.txt": "even-more-content", "cm2file2.txt": `apiVersion: v1
clusters:
- cluster:
    server: https://foo.local.gardener.cloud
  name: test
kind: Config
`},
					}
					secret1 = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: pod.Namespace},
						Data:       map[string][]byte{"secret1file1.txt": []byte("very-secret"), "secret1file2.txt": []byte("super-secret")},
					}
					secret2 = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: pod.Namespace},
						Data:       map[string][]byte{"secret2file1.txt": []byte("highly-secret"), "secret2file2.txt": []byte("please-dont-detect")},
					}
				)

				Expect(fakeClient.Create(ctx, configMap1)).To(Succeed())
				Expect(fakeClient.Create(ctx, configMap2)).To(Succeed())
				Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
				Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

				pod.Spec.Volumes = []corev1.Volume{
					{Name: "v1", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
					{Name: "v2", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "secret1"}}},
					{Name: "v3", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "cm2"}}},
							{Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{Name: "secret2"},
								Items:                []corev1.KeyToPath{{Key: "secret2file2.txt", Path: "mystery.txt"}},
							}},
						},
						DefaultMode: ptr.To[int32](0666),
					}}},
				}

				Expect(Translate(ctx, fakeClient, pod)).To(ConsistOf(
					extensionsv1alpha1.File{
						Path:        "/etc/kubernetes/manifests/" + pod.Name + ".yaml",
						Permissions: ptr.To[uint32](0640),
						Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
kind: Pod
metadata:
  annotations:
    bar: baz
  creationTimestamp: null
  labels:
    baz: foo
    static-pod: "true"
  name: foo
  namespace: kube-system
spec:
  containers: null
  hostAliases:
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: 127.0.0.1
  - hostnames:
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    ip: ::1
  hostNetwork: true
  priorityClassName: system-node-critical
  volumes:
  - hostPath:
      path: /var/lib/foo/v1
    name: v1
  - hostPath:
      path: /var/lib/foo/v2
    name: v2
  - hostPath:
      path: /var/lib/foo/v3
    name: v3
status: {}
`))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + pod.Name + "/" + pod.Spec.Volumes[0].Name + "/cm1file1.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(configMap1.Data["cm1file1.txt"]))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + pod.Name + "/" + pod.Spec.Volumes[0].Name + "/cm1file2.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(configMap1.Data["cm1file2.txt"]))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + pod.Name + "/" + pod.Spec.Volumes[1].Name + "/secret1file1.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(secret1.Data["secret1file1.txt"])}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + pod.Name + "/" + pod.Spec.Volumes[1].Name + "/secret1file2.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(secret1.Data["secret1file2.txt"])}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + pod.Name + "/" + pod.Spec.Volumes[2].Name + "/cm2file1.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(configMap2.Data["cm2file1.txt"]))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + pod.Name + "/" + pod.Spec.Volumes[2].Name + "/cm2file2.txt",
						Permissions: ptr.To[uint32](0640),
						Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
clusters:
- cluster:
    server: https://foo.local.gardener.cloud
  name: test
kind: Config
`))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/" + pod.Name + "/" + pod.Spec.Volumes[2].Name + "/mystery.txt",
						Permissions: ptr.To[uint32](0640),
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(secret2.Data["secret2file2.txt"])}},
					},
				))
			})
		})

		When("object is of unsupported type", func() {
			It("should return an error", func() {
				Expect(Translate(ctx, fakeClient, &corev1.ConfigMap{})).Error().To(MatchError(ContainSubstring("unsupported object type")))
			})
		})
	})
})
