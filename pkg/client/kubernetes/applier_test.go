// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"bytes"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var (
	configMapTypeMeta = metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"}
)

func mkManifest(objs ...client.Object) []byte {
	var out bytes.Buffer
	for _, obj := range objs {
		data, err := yaml.Marshal(obj)
		Expect(err).NotTo(HaveOccurred())
		out.Write(data)
		out.WriteString("---")
	}
	return out.Bytes()
}

var _ = Describe("Apply", func() {
	var (
		c       client.Client
		applier kubernetes.Applier
	)

	BeforeEach(func() {
		c = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
		mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)

		applier = kubernetes.NewApplier(c, mapper)
	})

	Context("ManifestTest", func() {
		var (
			rawConfigMap = []byte(`apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  name: test-cm
  namespace: test-ns`)
		)
		Context("manifest readers testing", func() {
			It("Should read manifest correctly", func() {
				unstructuredObject, err := kubernetes.NewManifestReader(rawConfigMap).Read()
				Expect(err).NotTo(HaveOccurred())

				// Tests to ensure validity of object
				Expect(unstructuredObject.GetName()).To(Equal("test-cm"))
				Expect(unstructuredObject.GetNamespace()).To(Equal("test-ns"))
			})
		})

		It("Should read manifest and swap namespace correctly", func() {
			unstructuredObject, err := kubernetes.NewNamespaceSettingReader(kubernetes.NewManifestReader(rawConfigMap), "swap-ns").Read()
			Expect(err).NotTo(HaveOccurred())

			// Tests to ensure validity of object and namespace swap
			Expect(unstructuredObject.GetName()).To(Equal("test-cm"))
			Expect(unstructuredObject.GetNamespace()).To(Equal("swap-ns"))
		})
	})

	Context("Applier", func() {
		var rawMultipleObjects = []byte(`
apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  name: test-cm
  namespace: test-ns
---
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: test-ns
spec:
  containers:
    - name: dns
      image: dnsutils`)

		Context("#ApplyManifest", func() {
			It("should create non-existent objects", func() {
				cm := corev1.ConfigMap{
					TypeMeta:   configMapTypeMeta,
					ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "n"},
				}
				manifest := mkManifest(&cm)
				manifestReader := kubernetes.NewManifestReader(manifest)
				Expect(applier.ApplyManifest(context.TODO(), manifestReader, kubernetes.DefaultMergeFuncs)).To(Succeed())

				var actualCM corev1.ConfigMap
				err := c.Get(context.TODO(), client.ObjectKey{Name: "c", Namespace: "n"}, &actualCM)
				Expect(err).NotTo(HaveOccurred())
				cm.SetResourceVersion("1")
				Expect(cm).To(DeepDerivativeEqual(actualCM))
			})

			It("should apply multiple objects", func() {
				manifestReader := kubernetes.NewManifestReader(rawMultipleObjects)
				Expect(applier.ApplyManifest(context.TODO(), manifestReader, kubernetes.DefaultMergeFuncs)).To(Succeed())

				err := c.Get(context.TODO(), client.ObjectKey{Name: "test-cm", Namespace: "test-ns"}, &corev1.ConfigMap{})
				Expect(err).NotTo(HaveOccurred())

				err = c.Get(context.TODO(), client.ObjectKey{Name: "test-pod", Namespace: "test-ns"}, &corev1.Pod{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should retain secret information for service account", func() {
				oldServiceAccount := corev1.ServiceAccount{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ServiceAccount",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-serviceaccount",
						Namespace: "test-ns",
					},
					Secrets: []corev1.ObjectReference{
						{
							Name: "test-secret",
						},
					},
				}
				newServiceAccount := oldServiceAccount
				newServiceAccount.Secrets = []corev1.ObjectReference{}
				manifest := mkManifest(&newServiceAccount)
				manifestReader := kubernetes.NewManifestReader(manifest)

				Expect(c.Create(context.TODO(), &oldServiceAccount)).To(Succeed())
				Expect(applier.ApplyManifest(context.TODO(), manifestReader, kubernetes.DefaultMergeFuncs)).To(Succeed())

				resultingService := &corev1.ServiceAccount{}
				err := c.Get(context.TODO(), client.ObjectKey{Name: "test-serviceaccount", Namespace: "test-ns"}, resultingService)
				Expect(err).NotTo(HaveOccurred())
				Expect(resultingService.Secrets).To(HaveLen(1))
				Expect(resultingService.Secrets[0].Name).To(Equal("test-secret"))
			})

			Context("DefaultMergeFuncs", func() {
				var (
					oldService *corev1.Service
					newService *corev1.Service
					expected   *corev1.Service
				)

				BeforeEach(func() {
					oldService = &corev1.Service{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Service",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-service",
							Namespace: "test-ns",
							Annotations: map[string]string{
								"loadbalancer.openstack.org/load-balancer-id": "09199d61-4cca-4c7d-8d9c-405ba7680dbe",
							},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "1.2.3.4",
							Type:      corev1.ServiceTypeClusterIP,
							Selector:  map[string]string{"foo": "bar"},
							Ports: []corev1.ServicePort{
								{
									Name:       "foo",
									Protocol:   corev1.ProtocolTCP,
									Port:       123,
									TargetPort: intstr.FromInt32(456),
								},
							},
						},
					}

					newService = oldService.DeepCopy()
					newService.Spec.ClusterIP = ""
					newService.Annotations = map[string]string{}
					expected = oldService.DeepCopy()
					expected.ResourceVersion = "2"
				})

				DescribeTable("Existing ClusterIP service",
					func(mutator func()) {
						mutator()
						Expect(c.Create(context.TODO(), oldService)).ToNot(HaveOccurred())

						manifest := mkManifest(newService)
						manifestReader := kubernetes.NewManifestReader(manifest)

						err := applier.ApplyManifest(context.TODO(), manifestReader, kubernetes.DefaultMergeFuncs)
						Expect(err).NotTo(HaveOccurred())

						result := &corev1.Service{}
						err = c.Get(context.TODO(), client.ObjectKey{Name: "test-service", Namespace: "test-ns"}, result)
						Expect(err).NotTo(HaveOccurred())

						Expect(result).To(Equal(expected))
					},

					Entry(
						"ClusterIP with changed ports", func() {
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)

							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
						}),
					Entry(
						"ClusterIP with changed ClusterIP, should not update it", func() {
							newService.Spec.ClusterIP = "5.6.7.8"
						}),
					Entry(
						"Headless ClusterIP", func() {
							newService.Spec.ClusterIP = "None"

							expected.Spec.ClusterIP = "None"
						}),
					Entry(
						"ClusterIP without passing any type, should update it", func() {
							newService.Spec.ClusterIP = "5.6.7.8"
							newService.Spec.Type = ""
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)

							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
						}),
					Entry(
						"NodePort with changed ports", func() {
							newService.Spec.Type = corev1.ServiceTypeNodePort
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 444

							expected.Spec.Type = corev1.ServiceTypeNodePort
							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 444
						}),
					Entry(
						"ExternalName removes ClusterIP", func() {
							newService.Spec.Type = corev1.ServiceTypeExternalName
							newService.Spec.Selector = nil
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 0
							newService.Spec.ClusterIP = ""
							newService.Spec.ExternalName = "foo.com"
							newService.Spec.HealthCheckNodePort = 0

							expected.Spec.Type = corev1.ServiceTypeExternalName
							expected.Spec.Selector = nil
							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 0
							expected.Spec.ClusterIP = ""
							expected.Spec.ExternalName = "foo.com"
							expected.Spec.HealthCheckNodePort = 0
						}),
				)

				DescribeTable("Existing NodePort service",
					func(mutator func()) {
						oldService.Spec.Ports[0].NodePort = 3333
						oldService.Spec.Type = corev1.ServiceTypeNodePort

						newService.Spec.Ports[0].NodePort = 3333
						newService.Spec.Type = corev1.ServiceTypeNodePort

						expected.Spec.Ports[0].NodePort = 3333
						expected.Spec.Type = corev1.ServiceTypeNodePort

						mutator()
						Expect(c.Create(context.TODO(), oldService)).ToNot(HaveOccurred())

						manifest := mkManifest(newService)
						manifestReader := kubernetes.NewManifestReader(manifest)

						err := applier.ApplyManifest(context.TODO(), manifestReader, kubernetes.DefaultMergeFuncs)
						Expect(err).NotTo(HaveOccurred())

						result := &corev1.Service{}
						err = c.Get(context.TODO(), client.ObjectKey{Name: "test-service", Namespace: "test-ns"}, result)
						Expect(err).NotTo(HaveOccurred())

						Expect(result).To(Equal(expected))
					},

					Entry(
						"ClusterIP with changed ports", func() {
							newService.Spec.Type = corev1.ServiceTypeClusterIP
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 0

							expected.Spec.Type = corev1.ServiceTypeClusterIP
							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 0
						}),
					Entry(
						"ClusterIP changed, should not update it", func() {
							newService.Spec.ClusterIP = "5.6.7.8"
						}),
					Entry(
						"Headless ClusterIP type service", func() {
							newService.Spec.Type = corev1.ServiceTypeClusterIP
							newService.Spec.ClusterIP = "None"

							expected.Spec.ClusterIP = "None"
							expected.Spec.Type = corev1.ServiceTypeClusterIP
						}),
					Entry(
						"NodePort with changed ports", func() {
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 444

							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 444
						}),
					Entry(
						"NodePort with changed ports and without nodePort", func() {
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 0

							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
						}),
					Entry(
						"ExternalName removes ClusterIP", func() {
							newService.Spec.Type = corev1.ServiceTypeExternalName
							newService.Spec.Selector = nil
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 0
							newService.Spec.ClusterIP = ""
							newService.Spec.ExternalName = "foo.com"
							newService.Spec.HealthCheckNodePort = 0

							expected.Spec.Type = corev1.ServiceTypeExternalName
							expected.Spec.Selector = nil
							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 0
							expected.Spec.ClusterIP = ""
							expected.Spec.ExternalName = "foo.com"
							expected.Spec.HealthCheckNodePort = 0
						}),
				)

				DescribeTable("Existing LoadBalancer service change to service of type",
					func(mutator func()) {
						oldService.Spec.Ports[0].NodePort = 3333
						oldService.Spec.Type = corev1.ServiceTypeLoadBalancer

						newService.Spec.Ports[0].NodePort = 3333
						newService.Spec.Type = corev1.ServiceTypeLoadBalancer

						expected.Spec.Ports[0].NodePort = 3333
						expected.Spec.Type = corev1.ServiceTypeLoadBalancer

						mutator()
						Expect(c.Create(context.TODO(), oldService)).ToNot(HaveOccurred())

						manifest := mkManifest(newService)
						manifestReader := kubernetes.NewManifestReader(manifest)

						err := applier.ApplyManifest(context.TODO(), manifestReader, kubernetes.DefaultMergeFuncs)
						Expect(err).NotTo(HaveOccurred())

						result := &corev1.Service{}
						err = c.Get(context.TODO(), client.ObjectKey{Name: "test-service", Namespace: "test-ns"}, result)
						Expect(err).NotTo(HaveOccurred())

						Expect(result).To(Equal(expected))
					},

					Entry(
						"ClusterIP with changed ports", func() {
							newService.Spec.Type = corev1.ServiceTypeClusterIP
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 0

							expected.Spec.Type = corev1.ServiceTypeClusterIP
							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 0
						}),
					Entry(
						"Cluster with ClusterIP changed, should not update it", func() {
							newService.Spec.ClusterIP = "5.6.7.8"
						}),
					Entry(
						"Headless ClusterIP type service", func() {
							newService.Spec.Type = corev1.ServiceTypeClusterIP
							newService.Spec.ClusterIP = "None"

							expected.Spec.ClusterIP = "None"
							expected.Spec.Type = corev1.ServiceTypeClusterIP
						}),
					Entry(
						"NodePort with changed ports", func() {
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 444

							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 444
						}),
					Entry(
						"NodePort with changed ports and without nodePort", func() {
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 0

							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
						}),
					Entry(
						"ExternalName removes ClusterIP", func() {
							newService.Spec.Type = corev1.ServiceTypeExternalName
							newService.Spec.Selector = nil
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 0
							newService.Spec.ClusterIP = ""
							newService.Spec.ExternalName = "foo.com"
							newService.Spec.HealthCheckNodePort = 0

							expected.Spec.Type = corev1.ServiceTypeExternalName
							expected.Spec.Selector = nil
							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 0
							expected.Spec.ClusterIP = ""
							expected.Spec.ExternalName = "foo.com"
							expected.Spec.HealthCheckNodePort = 0
						}),
					Entry(
						"LoadBalancer with ExternalTrafficPolicy=Local and HealthCheckNodePort", func() {
							newService.Spec.HealthCheckNodePort = 123
							newService.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal

							expected.Spec.HealthCheckNodePort = 123
							expected.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
						}),
					Entry(
						"LoadBalancer with ExternalTrafficPolicy=Local and no HealthCheckNodePort", func() {
							oldService.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
							oldService.Spec.HealthCheckNodePort = 3333

							newService.Spec.HealthCheckNodePort = 0
							newService.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal

							expected.Spec.HealthCheckNodePort = 3333
							expected.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
						}),
				)

				DescribeTable("Existing ExternalName service change to service of type",
					func(mutator func()) {
						oldService.Spec.Ports[0].NodePort = 0
						oldService.Spec.Type = corev1.ServiceTypeExternalName
						oldService.Spec.HealthCheckNodePort = 0
						oldService.Spec.ClusterIP = ""
						oldService.Spec.ExternalName = "baz.bar"
						oldService.Spec.Selector = nil

						newService.Spec.Ports[0].NodePort = 0
						newService.Spec.Type = corev1.ServiceTypeExternalName
						newService.Spec.HealthCheckNodePort = 0
						newService.Spec.ClusterIP = ""
						newService.Spec.ExternalName = "baz.bar"
						newService.Spec.Selector = nil

						expected.Spec.Ports[0].NodePort = 0
						expected.Spec.Type = corev1.ServiceTypeExternalName
						expected.Spec.HealthCheckNodePort = 0
						expected.Spec.ClusterIP = ""
						expected.Spec.ExternalName = "baz.bar"
						expected.Spec.Selector = nil

						mutator()
						Expect(c.Create(context.TODO(), oldService)).ToNot(HaveOccurred())

						manifest := mkManifest(newService)
						manifestReader := kubernetes.NewManifestReader(manifest)

						err := applier.ApplyManifest(context.TODO(), manifestReader, kubernetes.DefaultMergeFuncs)
						Expect(err).NotTo(HaveOccurred())

						result := &corev1.Service{}
						err = c.Get(context.TODO(), client.ObjectKey{Name: "test-service", Namespace: "test-ns"}, result)
						Expect(err).NotTo(HaveOccurred())

						Expect(result).To(Equal(expected))
					},

					Entry(
						"ClusterIP with changed ports", func() {
							newService.Spec.Type = corev1.ServiceTypeClusterIP
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 0
							newService.Spec.ExternalName = ""
							newService.Spec.ClusterIP = "3.4.5.6"

							expected.Spec.Type = corev1.ServiceTypeClusterIP
							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 0
							expected.Spec.ExternalName = ""
							expected.Spec.ClusterIP = "3.4.5.6"
						}),
					Entry(
						"NodePort with changed ports", func() {
							newService.Spec.Type = corev1.ServiceTypeNodePort
							newService.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							newService.Spec.Ports[0].Port = 999
							newService.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							newService.Spec.Ports[0].NodePort = 444
							newService.Spec.ExternalName = ""
							newService.Spec.ClusterIP = "3.4.5.6"

							expected.Spec.Type = corev1.ServiceTypeNodePort
							expected.Spec.Ports[0].Protocol = corev1.ProtocolUDP
							expected.Spec.Ports[0].Port = 999
							expected.Spec.Ports[0].TargetPort = intstr.FromInt32(888)
							expected.Spec.Ports[0].NodePort = 444
							expected.Spec.ExternalName = ""
							expected.Spec.ClusterIP = "3.4.5.6"
						}),
					Entry(
						"LoadBalancer with ExternalTrafficPolicy=Local and HealthCheckNodePort", func() {
							newService.Spec.Type = corev1.ServiceTypeLoadBalancer
							newService.Spec.HealthCheckNodePort = 123
							newService.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
							newService.Spec.ExternalName = ""
							newService.Spec.ClusterIP = "3.4.5.6"

							expected.Spec.Type = corev1.ServiceTypeLoadBalancer
							expected.Spec.HealthCheckNodePort = 123
							expected.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
							expected.Spec.ExternalName = ""
							expected.Spec.ClusterIP = "3.4.5.6"
						}),
				)
			})

			Context("DeploymentKeepReplicasMergeFunc", func() {
				var (
					oldDeployment *appsv1.Deployment
					newDeployment *appsv1.Deployment
					expected      *appsv1.Deployment
				)

				BeforeEach(func() {
					oldDeployment = &appsv1.Deployment{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Deployment",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-deploy",
							Namespace: "test-ns",
						},
					}

					newDeployment = oldDeployment.DeepCopy()
					expected = oldDeployment.DeepCopy()
					expected.ResourceVersion = "2"
				})

				DescribeTable("Existing Deployment",
					func(mutator func()) {
						mutator()
						Expect(c.Create(context.TODO(), oldDeployment)).ToNot(HaveOccurred())

						manifest := mkManifest(newDeployment)
						manifestReader := kubernetes.NewManifestReader(manifest)

						err := applier.ApplyManifest(context.TODO(), manifestReader, kubernetes.MergeFuncs{appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind(): kubernetes.DeploymentKeepReplicasMergeFunc})
						Expect(err).NotTo(HaveOccurred())

						result := &appsv1.Deployment{}
						err = c.Get(context.TODO(), client.ObjectKey{Name: "test-deploy", Namespace: "test-ns"}, result)
						Expect(err).NotTo(HaveOccurred())

						Expect(result).To(Equal(expected))
					},

					Entry(
						"old without replicas, new without replicas", func() {
							oldDeployment.Spec.Replicas = nil
							newDeployment.Spec.Replicas = nil
							expected.Spec.Replicas = nil
						},
					),
					Entry(
						"old without replicas, new with replicas", func() {
							oldDeployment.Spec.Replicas = nil
							newDeployment.Spec.Replicas = ptr.To[int32](2)
							expected.Spec.Replicas = ptr.To[int32](2)
						},
					),
					Entry(
						"old with replicas, new without replicas", func() {
							oldDeployment.Spec.Replicas = ptr.To[int32](3)
							newDeployment.Spec.Replicas = nil
							expected.Spec.Replicas = ptr.To[int32](3)
						},
					),
					Entry(
						"old with replicas, new with replicas", func() {
							oldDeployment.Spec.Replicas = ptr.To[int32](3)
							newDeployment.Spec.Replicas = ptr.To[int32](4)
							expected.Spec.Replicas = ptr.To[int32](3)
						},
					),
				)
			})

			It("should create objects with namespace", func() {
				cm := corev1.ConfigMap{
					TypeMeta:   configMapTypeMeta,
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"},
				}
				manifest := mkManifest(&cm)
				manifestReader := kubernetes.NewManifestReader(manifest)
				namespaceSettingReader := kubernetes.NewNamespaceSettingReader(manifestReader, "b")
				Expect(applier.ApplyManifest(context.TODO(), namespaceSettingReader, kubernetes.DefaultMergeFuncs)).To(Succeed())

				var actualCMWithNamespace corev1.ConfigMap
				err := c.Get(context.TODO(), client.ObjectKey{Name: "test", Namespace: "b"}, &actualCMWithNamespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualCMWithNamespace.Namespace).To(Equal("b"))
			})
		})

		Context("#DeleteManifest", func() {
			var result error

			BeforeEach(func() {
				existingServiceAccount := &corev1.ServiceAccount{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ServiceAccount",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-serviceaccount",
						Namespace: "test-ns",
					},
				}
				existingConfigMap := &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cm",
						Namespace: "test-ns",
					},
				}
				notDeletedConfigMap := &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "should-not-be-deleted-cm",
						Namespace: "test-ns",
					},
				}
				Expect(c.Create(context.TODO(), existingServiceAccount)).To(Succeed())
				Expect(c.Create(context.TODO(), existingConfigMap)).To(Succeed())
				Expect(c.Create(context.TODO(), notDeletedConfigMap)).To(Succeed())
				result = applier.DeleteManifest(context.TODO(), kubernetes.NewManifestReader(rawMultipleObjects))
			})

			It("should not return error", func() {
				Expect(result).NotTo(HaveOccurred())
			})

			It("should delete configmap", func() {
				err := c.Get(context.TODO(), client.ObjectKey{Name: "test-cm", Namespace: "test-ns"}, &corev1.ConfigMap{})
				Expect(err).To(BeNotFoundError())
			})

			It("should delete pod", func() {
				err := c.Get(context.TODO(), client.ObjectKey{Name: "test-pod", Namespace: "test-ns"}, &corev1.Pod{})
				Expect(err).To(BeNotFoundError())
			})

			It("should keep configmap which should not be deleted", func() {
				err := c.Get(context.TODO(), client.ObjectKey{Name: "should-not-be-deleted-cm", Namespace: "test-ns"}, &corev1.ConfigMap{})
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
