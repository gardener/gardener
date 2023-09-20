// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strconv"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/extensions/extension"
	. "github.com/gardener/gardener/pkg/component/shootsystem"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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
		kubernetesVersion = "1.25.1"
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
				Networking: &gardencorev1beta1.Networking{
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
						logr.Discard(),
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

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
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
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(pointer.Bool(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
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

			Context("k8s = 1.22", func() {
				BeforeEach(func() {
					values.Shoot.KubernetesVersion = semver.MustParse("1.22.6")
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					for _, name := range append(defaultKCMControllerSANames, "default", "endpointslicemirroring-controller", "ephemeral-volume-controller", "storage-version-garbage-collector", "service-controller", "route-controller", "node-controller") {
						Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__"+name+".yaml"])).To(Equal(serviceAccountYAMLFor(name)), name)
					}
				})
			})

			Context("k8s >= 1.26", func() {
				BeforeEach(func() {
					values.Shoot.KubernetesVersion = semver.MustParse("1.26.4")
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					for _, name := range append(defaultKCMControllerSANames, "default", "endpointslicemirroring-controller", "ephemeral-volume-controller", "storage-version-garbage-collector", "service-controller", "route-controller", "node-controller", "resource-claim-controller") {
						Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__"+name+".yaml"])).To(Equal(serviceAccountYAMLFor(name)), name)
					}
				})
			})

			Context("k8s >= 1.28", func() {
				BeforeEach(func() {
					values.Shoot.KubernetesVersion = semver.MustParse("1.28.2")
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					for _, name := range append(defaultKCMControllerSANames, "default", "endpointslicemirroring-controller", "ephemeral-volume-controller", "storage-version-garbage-collector", "service-controller", "route-controller", "node-controller", "resource-claim-controller", "legacy-service-account-token-cleaner", "validatingadmissionpolicy-status-controller") {
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

		Context("PriorityClasses", func() {
			It("should successfully deploy all well-known PriorityClasses", func() {
				expectPriorityClasses(managedResourceSecret.Data)
			})
		})

		Context("NetworkPolicies", func() {
			var (
				networkPolicyToAPIServer = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows traffic to the API server in TCP port 443 for
      pods labeled with 'networking.gardener.cloud/to-apiserver=allowed'.
  creationTimestamp: null
  name: gardener.cloud--allow-to-apiserver
  namespace: kube-system
spec:
  egress:
  - ports:
    - port: 443
      protocol: TCP
  podSelector:
    matchLabels:
      networking.gardener.cloud/to-apiserver: allowed
  policyTypes:
  - Egress
`
				networkPolicyToDNS = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows egress traffic from pods labeled with 'networking.gardener.cloud/to-dns=allowed'
      to DNS running in the 'kube-system' namespace.
  creationTimestamp: null
  name: gardener.cloud--allow-to-dns
  namespace: kube-system
spec:
  egress:
  - ports:
    - port: 8053
      protocol: UDP
    - port: 8053
      protocol: TCP
    to:
    - podSelector:
        matchExpressions:
        - key: k8s-app
          operator: In
          values:
          - kube-dns
  - ports:
    - port: 53
      protocol: UDP
    - port: 53
      protocol: TCP
    to:
    - ipBlock:
        cidr: 0.0.0.0/0
    - podSelector:
        matchExpressions:
        - key: k8s-app
          operator: In
          values:
          - node-local-dns
  podSelector:
    matchLabels:
      networking.gardener.cloud/to-dns: allowed
  policyTypes:
  - Egress
`
				networkPolicyToKubelet = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows egress traffic to kubelet in TCP port 10250
      for pods labeled with 'networking.gardener.cloud/to-kubelet=allowed'.
  creationTimestamp: null
  name: gardener.cloud--allow-to-kubelet
  namespace: kube-system
spec:
  egress:
  - ports:
    - port: 10250
      protocol: TCP
  podSelector:
    matchLabels:
      networking.gardener.cloud/to-kubelet: allowed
  policyTypes:
  - Egress
`
				networkPolicyToPublicNetworks = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows egress traffic to all networks for pods labeled
      with 'networking.gardener.cloud/to-public-networks=allowed'.
  creationTimestamp: null
  name: gardener.cloud--allow-to-public-networks
  namespace: kube-system
spec:
  egress:
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
  podSelector:
    matchLabels:
      networking.gardener.cloud/to-public-networks: allowed
  policyTypes:
  - Egress
`
			)

			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-to-apiserver.yaml"])).To(Equal(networkPolicyToAPIServer))
				Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-to-dns.yaml"])).To(Equal(networkPolicyToDNS))
				Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-to-kubelet.yaml"])).To(Equal(networkPolicyToKubelet))
				Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-to-public-networks.yaml"])).To(Equal(networkPolicyToPublicNetworks))
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
				})).To(Succeed())

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
				})).To(Succeed())

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

func expectPriorityClasses(data map[string][]byte) {
	expected := []struct {
		name        string
		value       int32
		description string
	}{
		{"gardener-shoot-system-900", 999999900, "PriorityClass for Shoot system components"},
		{"gardener-shoot-system-800", 999999800, "PriorityClass for Shoot system components"},
		{"gardener-shoot-system-700", 999999700, "PriorityClass for Shoot system components"},
		{"gardener-shoot-system-600", 999999600, "PriorityClass for Shoot system components"},
	}

	for _, pc := range expected {
		ExpectWithOffset(1, data).To(HaveKeyWithValue("priorityclass____"+pc.name+".yaml", []byte(`apiVersion: scheduling.k8s.io/v1
description: `+pc.description+`
kind: PriorityClass
metadata:
  creationTimestamp: null
  name: `+pc.name+`
value: `+strconv.FormatInt(int64(pc.value), 10)+`
`),
		))
	}
}
