// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
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
	"nginx-ingress",
	"fluent-bit",
	"fluent-operator",
	"fluent-operator-custom-resources-garden",
	"vali",
	"plutono",
	"prometheus-operator",
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
	"gardener-scheduler-runtime",
	"gardener-scheduler-virtual",
	"gardener-dashboard-runtime",
	"gardener-dashboard-virtual",
	"terminal-runtime",
	"terminal-virtual",
	"gardener-metrics-exporter-runtime",
	"gardener-metrics-exporter-virtual",
	"extension-admission-runtime-provider-local",
	"extension-admission-virtual-provider-local",
	"extension-registration-provider-local",
	"extension-provider-local-garden",
	"local-ext-shoot",
}

var istioManagedResourceList = []string{
	"istio-system",
	"virtual-garden-istio",
}

// ItShouldCreateGarden creates the garden object
func ItShouldCreateGarden(s *GardenContext) {
	GinkgoHelper()

	It("Create Garden", func(ctx SpecContext) {
		s.Log.Info("Creating Backup Secret")
		Eventually(ctx, func() error {
			if err := s.GardenClient.Create(ctx, s.BackupSecret); !apierrors.IsAlreadyExists(err) {
				return err
			}
			return StopTrying("backup secret already exists")
		}).Should(Succeed())

		s.Log.Info("Creating Garden")

		Eventually(ctx, func() error {
			if err := s.GardenClient.Create(ctx, s.Garden); !apierrors.IsAlreadyExists(err) {
				return err
			}
			return StopTrying("garden already exists")
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForGardenToBeReconciledAndHealthy waits for the garden to be reconciled successfully and healthy
func ItShouldWaitForGardenToBeReconciledAndHealthy(s *GardenContext) {
	GinkgoHelper()

	It("Wait for Garden to be reconciled", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) bool {
			g.Expect(s.GardenKomega.Get(s.Garden)()).To(Succeed())

			completed, reason := gardenReconciliationSuccessful(s.Garden)
			if !completed {
				s.Log.Info("Waiting for reconciliation and healthiness", "lastOperation", s.Garden.Status.LastOperation, "reason", reason)
			}
			return completed
		}).WithPolling(30 * time.Second).Should(BeTrue())

		s.Log.Info("Garden has been reconciled and is healthy")
	}, SpecTimeout(15*time.Minute))
}

// ItShouldAnnotateGarden sets the given annotation within the garden metadata to the specified value and patches the garden object
func ItShouldAnnotateGarden(s *GardenContext, annotations map[string]string) {
	GinkgoHelper()

	It("Annotate Garden", func(ctx SpecContext) {
		patch := client.MergeFrom(s.Garden.DeepCopy())

		for annotationKey, annotationValue := range annotations {
			s.Log.Info("Setting annotation", "annotation", annotationKey, "value", annotationValue)
			metav1.SetMetaDataAnnotation(&s.Garden.ObjectMeta, annotationKey, annotationValue)
		}

		Eventually(ctx, func() error {
			return s.GardenClient.Patch(ctx, s.Garden, patch)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldInitializeVirtualClusterClient initialized the contexts virtual cluster client from the "gardener" secret in the garden namespace
func ItShouldInitializeVirtualClusterClient(s *GardenContext) {
	GinkgoHelper()

	It("Initialize virtual cluster client", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			virtualClusterClient, err := kubernetes.NewClientFromSecret(ctx, s.GardenClient, v1beta1constants.GardenNamespace, "gardener",
				kubernetes.WithDisabledCachedClient(),
				kubernetes.WithClientOptions(client.Options{Scheme: operatorclient.VirtualScheme}),
			)
			g.Expect(err).NotTo(HaveOccurred())
			s.WithVirtualClusterClientSet(virtualClusterClient)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldVerifyGardenManagedResourcesAndAwaitHealthiness verifies that the managed resources in the "garden" namespace are the ones we expect and waits for their healthiness
func ItShouldVerifyGardenManagedResourcesAndAwaitHealthiness(s *GardenContext) {
	GinkgoHelper()
	itShouldVerifyManagedResourcesAndAwaitHealthiness(s, v1beta1constants.GardenNamespace, gardenManagedResourceList)
}

// ItShouldVerifyIstioManagedResourcesAndAwaitHealthiness verifies that the managed resources in the "istio-system" namespace are the ones we expect and waits for their healthiness
func ItShouldVerifyIstioManagedResourcesAndAwaitHealthiness(s *GardenContext) {
	GinkgoHelper()
	itShouldVerifyManagedResourcesAndAwaitHealthiness(s, v1beta1constants.IstioSystemNamespace, istioManagedResourceList)
}

func itShouldVerifyManagedResourcesAndAwaitHealthiness(s *GardenContext, namespace string, managedResourceNames []string) {
	managedResourceList := []resourcesv1alpha1.ManagedResource{}
	for _, managedResource := range managedResourceNames {
		managedResourceList = append(managedResourceList, resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResource,
				Namespace: namespace,
			},
		})
	}

	equalsManagedResourcesInNamespace(s, namespace, managedResourceList...)
	waitForManagedResourcesToBeHealthy(s, managedResourceList)
}

func equalsManagedResourcesInNamespace(s *GardenContext, namespace string, expectedManagedResources ...resourcesv1alpha1.ManagedResource) {
	It(fmt.Sprintf("Verify ManagedResources in namespace %s equal expected resources", namespace), func(ctx SpecContext) {
		managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
		Eventually(ctx, s.GardenKomega.List(managedResourceList, client.InNamespace(namespace))).Should(Succeed())
		Expect(managedResourceList.Items).To(ConsistOf(managedResourceNames(expectedManagedResources)))
	}, SpecTimeout(time.Minute))
}

func waitForManagedResourcesToBeHealthy(s *GardenContext, managedResourceList []resourcesv1alpha1.ManagedResource) {
	for _, managedResource := range managedResourceList {
		It(fmt.Sprintf("Wait for ManagedResource %s/%s to be healthy", managedResource.Namespace, managedResource.Name), func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(&managedResource), &managedResource)).To(Succeed())
				g.Expect(managedResource).To(beHealthyManagedResource())
			}).WithPolling(15 * time.Second).Should(Succeed())
		}, SpecTimeout(5*time.Minute))
	}
}

// ItShouldDeleteGarden deletes the garden object
func ItShouldDeleteGarden(s *GardenContext) {
	GinkgoHelper()

	It("Delete Garden", func(ctx SpecContext) {
		s.Log.Info("Deleting Garden")

		Eventually(ctx, func(g Gomega) {
			g.Expect(gardenerutils.ConfirmDeletion(ctx, s.GardenClient, s.Garden)).To(Succeed())
			g.Expect(s.GardenClient.Delete(ctx, s.Garden)).To(Succeed())
		}).Should(Succeed())

		s.Log.Info("Deleting Backup Secret")
		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenClient.Delete(ctx, s.BackupSecret)).To(Succeed())
		}).Should(Succeed())
	})
}

// ItShouldWaitForGardenToBeDeleted waits for the garden object to be gone
func ItShouldWaitForGardenToBeDeleted(s *GardenContext) {
	GinkgoHelper()

	It("Wait for Garden to be deleted", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			err := s.GardenKomega.Get(s.Garden)()
			if err == nil {
				s.Log.Info("Waiting for deletion", "lastOperation", s.Garden.Status.LastOperation)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		s.Log.Info("Garden has been deleted")
	}, SpecTimeout(15*time.Minute))
}

// ItShouldCleanUp cleans up any remaining volumes and etcd encryption configs
func ItShouldCleanUp(s *GardenContext) {
	itShouldCleanupVolumes(s)
	itShouldCleanupEtcdEncryptionConfig(s)
}

func itShouldCleanupVolumes(s *GardenContext) {
	GinkgoHelper()

	It("Delete all persistent volume claims in garden namespace", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, client.InNamespace(v1beta1constants.GardenNamespace))).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Wait for PersistentVolumes to be cleaned up", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) bool {
			pvList := &corev1.PersistentVolumeList{}
			g.Expect(s.GardenClient.List(ctx, pvList)).To(Succeed())

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
	}, SpecTimeout(time.Minute))
}

func itShouldCleanupEtcdEncryptionConfig(s *GardenContext) {
	GinkgoHelper()

	It("Delete etcd-encryption-configurations", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{"role": "kube-apiserver-etcd-encryption-configuration"})).To(Succeed())
			g.Expect(s.GardenClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{"role": "gardener-apiserver-etcd-encryption-configuration"})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForExtensionToReportDeletion waits for the specified extension to report DeleteSuccessful
func ItShouldWaitForExtensionToReportDeletion(s *GardenContext, extensionName string) {
	extension := &operatorv1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: extensionName,
		},
	}

	It(fmt.Sprintf("Wait for extension %s to report deletion", extensionName), func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
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
