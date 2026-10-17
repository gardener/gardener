// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

var gardenManagedResourceList = []string{
	"vpa",
	"etcd-druid",
	"kube-state-metrics-runtime",
	"kube-apiserver-sni",
	"istio-tls-secrets",
	"shoot-core-kube-controller-manager",
	"shoot-core-gardener-resource-manager",
	"shoot-core-gardeneraccess",
	"fluent-bit",
	"fluent-operator",
	"fluent-operator-custom-resources-garden",
	"vali",
	"victoria-logs",
	"plutono",
	"perses",
	"prometheus-operator",
	"perses-operator",
	"victoria-operator",
	"alertmanager-garden",
	"prometheus-garden",
	"prometheus-garden-target",
	"prometheus-longterm",
	"blackbox-exporter",
	"garden-system",
	"garden-system-virtual",
	"gardener-apiserver-runtime",
	"gardener-apiserver-virtual",
	"gardener-admission-controller-runtime",
	"gardener-admission-controller-virtual",
	"gardener-controller-manager-runtime",
	"gardener-controller-manager-virtual",
	"gardener-discovery-server-runtime",
	"gardener-discovery-server-virtual",
	"gardener-scheduler-runtime",
	"gardener-scheduler-virtual",
	"gardener-dashboard-runtime",
	"gardener-dashboard-virtual",
	"terminal-runtime",
	"terminal-virtual",
	"gardener-metrics-exporter-runtime",
	"gardener-metrics-exporter-virtual",
	"garden-extension-provider-local-45bc6",
	"extension-admission-runtime-provider-local",
	"extension-admission-virtual-provider-local",
	"extension-registration-provider-local",
	"extension-admission-runtime-networking-calico",
	"extension-admission-virtual-networking-calico",
	"extension-registration-networking-calico",
	"extension-admission-runtime-networking-cilium",
	"extension-admission-virtual-networking-cilium",
	"extension-registration-networking-cilium",
	"local-ext-shoot",
	"opentelemetry-operator",
	"opentelemetry-collector",
	"virtual-garden-istio-basic-auth-server",
}

var istioManagedResourceList = []string{
	"istio-system",
	"virtual-garden-istio",
}

// ItShouldCreateGarden creates the garden object
func ItShouldCreateGarden(tc *GardenContext) {
	GinkgoHelper()

	It("Create Garden", func(ctx SpecContext) {
		tc.Log.Info("Creating Backup Secret")
		Eventually(ctx, func() error {
			if err := tc.GardenClient.Create(ctx, backupSecretForGarden(tc.Garden)); !apierrors.IsAlreadyExists(err) {
				return err
			}
			return StopTrying("backup secret already exists")
		}).Should(Succeed())

		tc.Log.Info("Creating Garden")

		Eventually(ctx, func() error {
			if err := tc.GardenClient.Create(ctx, tc.Garden); !apierrors.IsAlreadyExists(err) {
				return err
			}
			return StopTrying("garden already exists")
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForGardenToBeReconciledAndHealthy waits for the garden to be reconciled successfully and healthy
func ItShouldWaitForGardenToBeReconciledAndHealthy(tc *GardenContext) {
	GinkgoHelper()

	It("Wait for Garden to be reconciled", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) bool {
			g.Expect(tc.GardenKomega.Get(tc.Garden)()).To(Succeed())

			completed, reason := gardenReconciliationSuccessful(tc.Garden)
			if !completed {
				tc.Log.Info("Waiting for reconciliation and healthiness", "lastOperation", tc.Garden.Status.LastOperation, "reason", reason)
			}
			return completed
		}).WithPolling(30 * time.Second).Should(BeTrue())

		tc.Log.Info("Garden has been reconciled and is healthy")
	}, SpecTimeout(15*time.Minute))
}

// ItShouldAnnotateGarden sets the given annotation within the garden metadata to the specified value and patches the garden object
func ItShouldAnnotateGarden(tc *GardenContext, annotations map[string]string) {
	GinkgoHelper()

	It("Annotate Garden", func(ctx SpecContext) {
		patch := client.MergeFrom(tc.Garden.DeepCopy())

		for key, value := range annotations {
			tc.Log.Info("Setting annotation", "annotation", key, "value", value)
			metav1.SetMetaDataAnnotation(&tc.Garden.ObjectMeta, key, value)
		}

		Eventually(ctx, func() error {
			return tc.GardenClient.Patch(ctx, tc.Garden, patch)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldInitializeVirtualClusterClient initialized the contexts virtual cluster client from the "gardener" secret in the garden namespace
func ItShouldInitializeVirtualClusterClient(tc *GardenContext) {
	GinkgoHelper()

	It("Initialize virtual cluster client", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			virtualClusterClient, err := kubernetes.NewClientFromSecret(ctx, tc.GardenClient, v1beta1constants.GardenNamespace, "gardener",
				kubernetes.WithDisabledCachedClient(),
				kubernetes.WithClientOptions(client.Options{Scheme: operatorclient.VirtualScheme}),
			)
			g.Expect(err).NotTo(HaveOccurred())
			tc.SetVirtualClusterClientSet(virtualClusterClient)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldVerifyGardenManagedResourcesAndAwaitHealthiness verifies that the managed resources in the "garden" namespace are the ones we expect and waits for their healthiness
func ItShouldVerifyGardenManagedResourcesAndAwaitHealthiness(tc *GardenContext) {
	GinkgoHelper()
	itShouldVerifyManagedResourcesAndAwaitHealthiness(tc, v1beta1constants.GardenNamespace, gardenManagedResourceList)
}

// ItShouldVerifyIstioManagedResourcesAndAwaitHealthiness verifies that the managed resources in the "istio-system" namespace are the ones we expect and waits for their healthiness
func ItShouldVerifyIstioManagedResourcesAndAwaitHealthiness(tc *GardenContext) {
	GinkgoHelper()
	itShouldVerifyManagedResourcesAndAwaitHealthiness(tc, v1beta1constants.IstioSystemNamespace, istioManagedResourceList)
}

func itShouldVerifyManagedResourcesAndAwaitHealthiness(tc *GardenContext, namespace string, managedResourceNames []string) {
	managedResourceList := []resourcesv1alpha1.ManagedResource{}
	for _, managedResource := range managedResourceNames {
		managedResourceList = append(managedResourceList, resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResource,
				Namespace: namespace,
			},
		})
	}

	equalsManagedResourcesInNamespace(tc, namespace, managedResourceList...)
	waitForManagedResourcesToBeHealthy(tc, managedResourceList)
}

func equalsManagedResourcesInNamespace(tc *GardenContext, namespace string, expectedManagedResources ...resourcesv1alpha1.ManagedResource) {
	It(fmt.Sprintf("Verify ManagedResources in namespace %s equal expected resources", namespace), func(ctx SpecContext) {
		managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
		Eventually(ctx, tc.GardenKomega.List(managedResourceList, client.InNamespace(namespace))).Should(Succeed())
		Expect(managedResourceList.Items).To(ConsistOf(managedResourceNames(expectedManagedResources)))
	}, SpecTimeout(time.Minute))
}

func waitForManagedResourcesToBeHealthy(tc *GardenContext, managedResourceList []resourcesv1alpha1.ManagedResource) {
	for _, managedResource := range managedResourceList {
		It(fmt.Sprintf("Wait for ManagedResource %s/%s to be healthy", managedResource.Namespace, managedResource.Name), func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(&managedResource), &managedResource)).To(Succeed())
				g.Expect(managedResource).To(beHealthyManagedResource())
			}).WithPolling(15 * time.Second).Should(Succeed())
		}, SpecTimeout(5*time.Minute))
	}
}

// ItShouldDeleteGarden deletes the garden object
func ItShouldDeleteGarden(tc *GardenContext) {
	GinkgoHelper()

	It("Delete Garden", func(ctx SpecContext) {
		tc.Log.Info("Deleting Garden")

		Eventually(ctx, func(g Gomega) {
			g.Expect(gardenerutils.ConfirmDeletion(ctx, tc.GardenClient, tc.Garden)).To(Succeed())
			g.Expect(tc.GardenClient.Delete(ctx, tc.Garden)).To(Succeed())
		}).Should(Succeed())

		tc.Log.Info("Deleting Backup Secret")
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenClient.Delete(ctx, backupSecretForGarden(tc.Garden))).To(Succeed())
		}).Should(Succeed())
	})
}

// ItShouldWaitForGardenToBeDeleted waits for the garden object to be gone
func ItShouldWaitForGardenToBeDeleted(tc *GardenContext) {
	GinkgoHelper()

	It("Wait for Garden to be deleted", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			err := tc.GardenKomega.Get(tc.Garden)()
			if err == nil {
				tc.Log.Info("Waiting for deletion", "lastOperation", tc.Garden.Status.LastOperation)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		tc.Log.Info("Garden has been deleted")
	}, SpecTimeout(15*time.Minute))
}

// ItShouldCleanUp cleans up any remaining volumes and etcd encryption configs
func ItShouldCleanUp(tc *GardenContext) {
	itShouldCleanupVolumes(tc)
	itShouldCleanupEtcdEncryptionConfig(tc)
}

func itShouldCleanupVolumes(tc *GardenContext) {
	GinkgoHelper()

	It("Delete all persistent volume claims in garden namespace", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, client.InNamespace(v1beta1constants.GardenNamespace))).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Wait for PersistentVolumes to be cleaned up", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) bool {
			pvList := &corev1.PersistentVolumeList{}
			g.Expect(tc.GardenClient.List(ctx, pvList)).To(Succeed())

			for _, pv := range pvList.Items {
				if pv.Spec.ClaimRef != nil &&
					pv.Spec.ClaimRef.APIVersion == "v1" &&
					pv.Spec.ClaimRef.Kind == "PersistentVolumeClaim" &&
					pv.Spec.ClaimRef.Namespace == v1beta1constants.GardenNamespace {
					return false
				}
			}

			return true
		}).WithPolling(2 * time.Second).Should(BeTrue())
	}, SpecTimeout(5*time.Minute))
}

func itShouldCleanupEtcdEncryptionConfig(tc *GardenContext) {
	GinkgoHelper()

	It("Delete etcd-encryption-configurations", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{"role": "kube-apiserver-etcd-encryption-configuration"})).To(Succeed())
			g.Expect(tc.GardenClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{"role": "gardener-apiserver-etcd-encryption-configuration"})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForExtensionToReportDeletion waits for the specified extension to report DeleteSuccessful
func ItShouldWaitForExtensionToReportDeletion(tc *GardenContext, extensionName string) {
	extension := &operatorv1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: extensionName,
		},
	}

	It(fmt.Sprintf("Wait for extension %s to report deletion", extensionName), func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
			g.Expect(extension.Status.Conditions).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionInstalled),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("DeleteSuccessful"),
			))
		}).WithPolling(2 * time.Second).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

func gardenReconciliationSuccessful(garden *operatorv1alpha1.Garden) (bool, string) {
	if garden.Generation != garden.Status.ObservedGeneration {
		return false, "garden generation did not equal observed generation"
	}
	if len(garden.Status.Conditions) == 0 && garden.Status.LastOperation == nil {
		return false, "no conditions and last operation present yet"
	}

	for _, condition := range garden.Status.Conditions {
		if condition.Status != gardencorev1beta1.ConditionTrue {
			return false, fmt.Sprintf("condition type %s is not true yet, had message %s with reason %s", condition.Type, condition.Message, condition.Reason)
		}
	}

	if garden.Status.LastOperation != nil {
		if garden.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
			return false, "last operation state is not succeeded"
		}
	}

	return true, ""
}

func managedResourceNames(managedResourceList []resourcesv1alpha1.ManagedResource) []gomegatypes.GomegaMatcher {
	out := []gomegatypes.GomegaMatcher{}

	for _, managedResource := range managedResourceList {
		out = append(out, MatchFields(IgnoreExtras, Fields{
			"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal(managedResource.Name)}),
		}))
	}

	return out
}

func beHealthyManagedResource() gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{"Conditions": And(
			ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue)),
			ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionTrue)),
			ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse)),
		)}),
	})
}

// ItShouldCreatePrometheusRuleForGarden creates a PrometheusRule and makes sure it is created.
func ItShouldCreatePrometheusRuleForGarden(tc *GardenContext, rule *monitoringv1.PrometheusRule) {
	GinkgoHelper()

	It("Create PrometheusRule "+rule.Namespace+"/"+rule.Name, func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenClient.Create(ctx, rule)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldDeletePrometheusRuleForGarden deletes a PrometheusRule and makes sure it is deleted.
func ItShouldDeletePrometheusRuleForGarden(tc *GardenContext, rule *monitoringv1.PrometheusRule) {
	GinkgoHelper()

	It("Delete PrometheusRule "+rule.Namespace+"/"+rule.Name, func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenClient.Delete(ctx, rule)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}
