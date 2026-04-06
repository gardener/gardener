// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dataplanedeployment_test

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/opentelemetry/dataplanedeployment"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("DataplaneDeployment", func() {
	var (
		ctx = context.Background()

		namespace = "some-namespace"
		image     = "some-otel-image:some-tag"

		c            client.Client
		component    component.DeployWaiter
		consistOf    func(...client.Object) types.GomegaMatcher
		customValues Values

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		fakeOps *retryfake.Ops
	)

	BeforeEach(func() {
		format.MaxDepth = 100000
		format.MaxLength = 100000
		format.TruncateThreshold = 100000
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		customValues = Values{
			Image:    image,
			Replicas: 1,
		}

		component = New(c, namespace, customValues)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-otel-collector-dataplane",
				Namespace: namespace,
			},
		}

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-shoot-core-otel-collector-dataplane",
				Namespace: namespace,
			},
		}

		fakeOps = &retryfake.Ops{MaxAttempts: 1}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())

			utilruntime.Must(references.InjectAnnotations(managedResource))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(managedResource).To(consistOf(
				getOtelCollectorDataplaneServiceAccount(),
				getOtelCollectorDataplaneClusterRole(),
				getOtelCollectorDataplaneClusterRoleBinding(),
				getOtelCollectorDataplaneService(),
				getOtelCollectorDataplaneConfigMap(),
				getOtelCollectorDataplaneDeployment(image),
			))
		})
	})

	Describe("#Destroy", func() {
		It("should delete managed resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Describe("#Wait", func() {
		It("should fail when MR does not exist", func() {
			Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should succeed when MR is healthy", func() {
			Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       managedResource.Name,
					Namespace:  namespace,
					Generation: 1,
				},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
						{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			})).To(Succeed())

			Expect(component.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should succeed when deleted", func() {
			Expect(component.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

func getOtelCollectorDataplaneServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-dataplane-deployment",
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
		},
		AutomountServiceAccountToken: ptr.To(true),
	}
}

func getOtelCollectorDataplaneClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "otel-collector-dataplane-deployment",
			Labels: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"nodes",
					"services",
					"endpoints",
					"pods",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{
					"nodes/metrics",
				},
				Verbs: []string{"get"},
			},
		},
	}
}

func getOtelCollectorDataplaneClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "otel-collector-dataplane-deployment",
			Labels: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "otel-collector-dataplane-deployment",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "otel-collector-dataplane-deployment",
				Namespace: "kube-system",
			},
		},
	}
}

func getOtelCollectorDataplaneService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-dataplane-deployment",
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "metrics",
					Port:     8080,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}

func getOtelCollectorDataplaneConfigMap() *corev1.ConfigMap {
	const configPath = "testdata/opentelemetry-collector-dataplane-config.test.yaml"

	data, err := os.ReadFile(configPath)
	if err != nil {
		panic(fmt.Sprintf("failed to read config file %s: %v", configPath, err))
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-dataplane-deployment",
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
		},
		Data: map[string]string{
			"config.yaml": string(data),
		},
	}
}
func getOtelCollectorDataplaneDeployment(image string) *appsv1.Deployment {
	replicas := int32(1)
	revHistory := int32(2)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-dataplane-deployment",
			Namespace: "kube-system",
			Labels: map[string]string{
				"component":           "otel-collector-dataplane-deployment",
				"gardener.cloud/role": "monitoring",
				"origin":              "gardener",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &replicas,
			RevisionHistoryLimit: &revHistory,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "otel-collector-dataplane-deployment",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"component":                                  "otel-collector-dataplane-deployment",
						"gardener.cloud/role":                        "monitoring",
						"networking.gardener.cloud/from-seed":        "allowed",
						"networking.gardener.cloud/to-apiserver":     "allowed",
						"networking.gardener.cloud/to-dns":           "allowed",
						"networking.gardener.cloud/to-kubelet":       "allowed",
						"networking.gardener.cloud/to-node-exporter": "allowed",
						"origin": "gardener",
					},
				},
				Spec: corev1.PodSpec{
					PriorityClassName:  "gardener-shoot-system-700",
					ServiceAccountName: "otel-collector-dataplane-deployment",
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To(int64(65534)),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "otel-collector-dataplane-deployment",
							Image:   image,
							Command: []string{"/bin/otelcol", "--config=/etc/otel-collector/config.yaml"},
							Ports: []corev1.ContainerPort{
								{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem:   ptr.To(true),
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									ReadOnly:  true,
									MountPath: "/etc/otel-collector",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "otel-collector-dataplane-deployment",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
