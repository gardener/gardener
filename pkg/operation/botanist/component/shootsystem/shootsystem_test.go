// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootsystem_test

import (
	"context"
	"net"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/extension"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/shootsystem"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ShootSystem", func() {
	var (
		ctx = context.TODO()

		managedResourceName = "shoot-core-system"
		namespace           = "some-namespace"

		projectName       = "foo"
		shootNamespace    = "garden-" + projectName
		shootName         = "bar"
		region            = "test-region"
		providerType      = "test-provider"
		kubernetesVersion = "1.17.1"
		maintenanceBegin  = "123456+0100"
		maintenanceEnd    = "134502+0100"
		domain            = "my-shoot.example.com"
		podCIDR           = "10.10.0.0/16"
		serviceCIDR       = "11.11.0.0/16"
		nodeCIDR          = "12.12.0.0/16"
		extension1        = "some-extension"
		extension2        = "some-other-extension"
		extensions        = map[string]extension.Extension{
			extension1: {},
			extension2: {},
		}
		shootObj = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: kubernetesVersion,
				},
				Provider: gardencorev1beta1.Provider{
					Type: providerType,
				},
				Maintenance: &gardencorev1beta1.Maintenance{
					TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
						Begin: maintenanceBegin,
						End:   maintenanceEnd,
					},
				},
				Networking: gardencorev1beta1.Networking{
					Nodes: &nodeCIDR,
				},
				Region: region,
			},
		}
		shoot *shootpkg.Shoot

		c         client.Client
		values    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	BeforeEach(func() {
		shoot = &shootpkg.Shoot{
			Components: &shootpkg.Components{
				Extensions: &shootpkg.Extensions{
					Extension: extension.New(
						nil,
						nil,
						&extension.Values{
							Extensions: extensions,
						},
						0,
						0,
						0,
					),
				},
			},
			ExternalClusterDomain: &domain,
			KubernetesVersion:     semver.MustParse(kubernetesVersion),
			Networks: &shootpkg.Networks{
				Pods:     parseCIDR(podCIDR),
				Services: parseCIDR(serviceCIDR),
			},
		}
		shoot.SetInfo(shootObj)

		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			ProjectName: projectName,
			Shoot:       shoot,
		}
		component = New(c, namespace, values)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.BoolPtr(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
		})

		Context("kube-controller-manager ServiceAccounts", func() {
			var (
				serviceAccountYAMLFor = func(name string) string {
					return `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  annotations:
    resources.gardener.cloud/keep-object: "true"
  creationTimestamp: null
  name: ` + name + `
  namespace: kube-system
`
				}
				defaultKCMControllerSANames = []string{"attachdetach-controller",
					"bootstrap-signer",
					"certificate-controller",
					"clusterrole-aggregation-controller",
					"controller-discovery",
					"cronjob-controller",
					"daemon-set-controller",
					"deployment-controller",
					"disruption-controller",
					"endpoint-controller",
					"endpointslice-controller",
					"expand-controller",
					"generic-garbage-collector",
					"horizontal-pod-autoscaler",
					"job-controller",
					"metadata-informers",
					"namespace-controller",
					"persistent-volume-binder",
					"pod-garbage-collector",
					"pv-protection-controller",
					"pvc-protection-controller",
					"replicaset-controller",
					"replication-controller",
					"resourcequota-controller",
					"root-ca-cert-publisher",
					"service-account-controller",
					"shared-informers",
					"statefulset-controller",
					"token-cleaner",
					"tokens-controller",
					"ttl-after-finished-controller",
					"ttl-controller",
				}
			)

			Context("k8s < 1.19", func() {
				It("should successfully deploy all resources", func() {
					for _, name := range append(defaultKCMControllerSANames, "default") {
						Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__"+name+".yaml"])).To(Equal(serviceAccountYAMLFor(name)), name)
					}
				})
			})

			Context("1.19 <= k8s < 1.20", func() {
				BeforeEach(func() {
					values.Shoot.KubernetesVersion = semver.MustParse("1.19.4")
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					for _, name := range append(defaultKCMControllerSANames, "default", "endpointslicemirroring-controller", "ephemeral-volume-controller") {
						Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__"+name+".yaml"])).To(Equal(serviceAccountYAMLFor(name)), name)
					}
				})
			})

			Context("k8s >= 1.20", func() {
				BeforeEach(func() {
					values.Shoot.KubernetesVersion = semver.MustParse("1.20.4")
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					for _, name := range append(defaultKCMControllerSANames, "default", "endpointslicemirroring-controller", "ephemeral-volume-controller", "storage-version-garbage-collector") {
						Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__"+name+".yaml"])).To(Equal(serviceAccountYAMLFor(name)), name)
					}
				})
			})

			Context("k8s >= 1.21", func() {
				BeforeEach(func() {
					values.Shoot.KubernetesVersion = semver.MustParse("1.21.4")
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					for _, name := range append(defaultKCMControllerSANames, "default", "endpointslicemirroring-controller", "ephemeral-volume-controller", "storage-version-garbage-collector", "service-controller", "route-controller", "node-controller") {
						Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__"+name+".yaml"])).To(Equal(serviceAccountYAMLFor(name)), name)
					}
				})
			})
		})

		Context("shoot-info ConfigMap", func() {
			configMap := `apiVersion: v1
data:
  domain: ` + domain + `
  extensions: ` + extension1 + `,` + extension2 + `
  kubernetesVersion: ` + kubernetesVersion + `
  maintenanceBegin: ` + maintenanceBegin + `
  maintenanceEnd: ` + maintenanceEnd + `
  nodeNetwork: ` + nodeCIDR + `
  podNetwork: ` + podCIDR + `
  projectName: ` + projectName + `
  provider: ` + providerType + `
  region: ` + region + `
  serviceNetwork: ` + serviceCIDR + `
  shootName: ` + shootName + `
kind: ConfigMap
metadata:
  creationTimestamp: null
  name: shoot-info
  namespace: kube-system
`

			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["configmap__kube-system__shoot-info.yaml"])).To(Equal(configMap))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				}))

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				}))

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

func parseCIDR(cidr string) *net.IPNet {
	_, ipNet, err := net.ParseCIDR(cidr)
	Expect(err).NotTo(HaveOccurred())
	return ipNet
}
