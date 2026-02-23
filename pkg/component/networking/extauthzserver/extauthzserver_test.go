// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extauthzserver_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	comp "github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/networking/extauthzserver"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ExtAuthzServer", func() {
	var (
		ctx = context.Background()

		managedResourceName = "ext-authz-server"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"
		priorityClassName   = "some-priority-class"
		secret1             = "first-secret"
		secret2             = "second-secret"
		prefix1             = "first"
		prefix2             = "second"
		prefix3             = "third"
		host1               = prefix1 + ".long.domain.name"
		host2               = prefix2 + ".even.longer.domain.name"
		host3               = prefix3 + ".domain.name"

		c                 client.Client
		component         comp.DeployWaiter
		fakeSecretManager secretsmanager.Interface
		values            Values

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		deployment = func(isGarden bool, prefixToSecret []prefixToSecretMapping) string {
			volumes := ""
			volumeMounts := ""
			for _, entry := range prefixToSecret {
				prefix := entry.prefix
				secret := entry.secret
				volumes += `      - name: ` + prefix + `
        secret:
          items:
          - key: auth
            path: ` + prefix + `
          secretName: ` + secret + `
`
				volumeMounts += `        - mountPath: /secrets/` + prefix + `
          name: ` + prefix + `
          subPath: ` + prefix + `
`
			}
			prefix := ""
			if isGarden {
				prefix = "virtual-garden-"
			}
			return `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: ` + prefix + `ext-authz-server
  name: ` + prefix + `ext-authz-server
  namespace: some-namespace
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ` + prefix + `ext-authz-server
  strategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: ` + prefix + `ext-authz-server
    spec:
      containers:
      - args:
        - --grpc-reflection
        image: some-image:some-tag
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 2
          periodSeconds: 10
          successThreshold: 1
          tcpSocket:
            port: 10000
          timeoutSeconds: 5
        name: ext-authz-server
        ports:
        - containerPort: 10000
          name: grpc
        readinessProbe:
          failureThreshold: 2
          periodSeconds: 10
          successThreshold: 1
          tcpSocket:
            port: 10000
          timeoutSeconds: 5
        resources:
          requests:
            cpu: 5m
            memory: 16Mi
        volumeMounts:
        - mountPath: /tls
          name: tls-server-certificate
          readOnly: true
` + volumeMounts + `      priorityClassName: some-priority-class
      volumes:
      - name: tls-server-certificate
        secret:
          secretName: ` + prefix + `ext-authz-server
` + volumes + `status: {}
`
		}
		envoyFilter = func(isGarden bool, hosts ...string) string {
			prefix := ""
			if isGarden {
				prefix = "virtual-garden-"
			}
			spec := ` {}
`
			if len(hosts) > 0 {
				spec = `
  configPatches:
`
				for _, host := range hosts {
					spec += `  - applyTo: HTTP_FILTER
    match:
      context: GATEWAY
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.http_connection_manager
          sni: ` + host + `
        portNumber: 9443
    patch:
      filterClass: AUTHZ
      operation: INSERT_BEFORE
      value:
        name: envoy.filters.http.ext_authz
        typed_config:
          '@type': type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
          grpc_service:
            envoy_grpc:
              cluster_name: outbound|10000||` + prefix + `ext-authz-server.some-namespace.svc.cluster.local
            timeout: 2s
          transport_api_version: V3
`
				}
			}
			return `apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: some-namespace-` + prefix + `ext-authz-server
  namespace: ` + prefix + `istio-ingress
  ownerReferences:
  - apiVersion: v1
    blockOwnerDeletion: true
    kind: Namespace
    name: some-namespace
    uid: ""
spec:` + spec + `status: {}
`
		}
		dummyVirtualService = &istionetworkingv1beta1.VirtualService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dummy-virtual-service",
				Namespace: namespace,
				Labels:    map[string]string{"app": "dummy"},
			},
		}
		realVirtualService1 = &istionetworkingv1beta1.VirtualService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "real-virtual-service-1",
				Namespace: namespace,
				Labels: map[string]string{
					"app": "dummy",
					"reference.gardener.cloud/basic-auth-secret-name": secret1,
				},
			},
			Spec: istioapinetworkingv1beta1.VirtualService{
				Hosts: []string{host1},
			},
		}
		realVirtualService2 = &istionetworkingv1beta1.VirtualService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "real-virtual-service-2",
				Namespace: namespace,
				Labels: map[string]string{
					"app": "dummy",
					"reference.gardener.cloud/basic-auth-secret-name": secret2,
				},
			},
			Spec: istioapinetworkingv1beta1.VirtualService{
				Hosts: []string{host2, host3},
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(c, namespace)

		values = Values{
			Image:             image,
			PriorityClassName: priorityClassName,
			Replicas:          int32(1),
		}
		component = New(c, namespace, fakeSecretManager, values)

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
		var (
			nonSecretManifests []string
			secretManifests    []*corev1.Secret
		)

		JustBeforeEach(func() {
			if values.IsGardenCluster {
				managedResource.Name = "virtual-garden-" + managedResourceName
				managedResourceSecret.Name = "managedresource-" + managedResource.Name
			}
			Expect(c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())
			nonSecretManifests, secretManifests = filterManifests(manifests)
		})

		testManifests := func(isGarden bool, prefixToSecret []prefixToSecretMapping, hosts ...string) {
			expectedManifests := []string{
				deployment(isGarden, prefixToSecret),
				envoyFilter(isGarden, hosts...),
			}
			Expect(nonSecretManifests).To(ConsistOf(expectedManifests))
			Expect(secretManifests).To(HaveLen(0))
		}

		Context("When no corresponding secrets exist", func() {
			Context("Shoot or Seed", func() {
				It("should successfully deploy the resources", func() {
					testManifests(false, []prefixToSecretMapping{})
				})
			})

			Context("Garden", func() {
				BeforeEach(func() {
					values.IsGardenCluster = true
					component = New(c, namespace, fakeSecretManager, values)
				})

				It("should successfully deploy the resources", func() {
					testManifests(true, []prefixToSecretMapping{})
				})
			})
		})

		Context("With secrets present", func() {
			BeforeEach(func() {
				Expect(c.Create(ctx, dummyVirtualService.DeepCopy())).To(Succeed())
				Expect(c.Create(ctx, realVirtualService1.DeepCopy())).To(Succeed())
				Expect(c.Create(ctx, realVirtualService2.DeepCopy())).To(Succeed())
			})

			Context("Shoot or Seed", func() {
				It("should successfully deploy the resources", func() {
					testManifests(false, []prefixToSecretMapping{
						{prefix1, secret1},
						{prefix2, secret2},
						{prefix3, secret2},
					}, host1, host2, host3)
				})
			})

			Context("Garden", func() {
				BeforeEach(func() {
					values.IsGardenCluster = true
					component = New(c, namespace, fakeSecretManager, values)
				})

				It("should successfully deploy the resources", func() {
					testManifests(true, []prefixToSecretMapping{
						{prefix1, secret1},
						{prefix2, secret2},
						{prefix3, secret2},
					}, host1, host2, host3)
				})
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

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var fakeOps *retryfake.Ops

		BeforeEach(func() {
			component = New(c, namespace, fakeSecretManager, values)
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
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

func filterManifests(manifests []string) (manifestsWithoutSecrets []string, secretManifests []*corev1.Secret) {
	for _, manifest := range manifests {
		secret, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode([]byte(manifest), nil, &corev1.Secret{})
		if err != nil {
			manifestsWithoutSecrets = append(manifestsWithoutSecrets, manifest)
		} else {
			secretManifests = append(secretManifests, secret.(*corev1.Secret))
		}
	}
	return manifestsWithoutSecrets, secretManifests
}

type prefixToSecretMapping struct {
	prefix string
	secret string
}
