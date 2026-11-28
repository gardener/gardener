// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/sprig/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

var seedTemplatesDir = filepath.Join("..", "..", "framework", "resources", "templates")

// ItShouldInitializeSeedClient initializes the context's seed clients from the garden/seed-<name> kubeconfig secret.
// Requires ItShouldGetResponsibleSeed to be called first.
func ItShouldInitializeSeedClient(tc *SeedContext) {
	GinkgoHelper()

	It("Initialize Seed client", func(ctx SpecContext) {
		Expect(tc.Seed).NotTo(BeNil(), "ItShouldGetResponsibleSeed should be called first")

		seedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "seed-" + tc.Seed.Name,
				Namespace: "garden",
			},
		}
		Eventually(ctx, tc.GardenKomega.Object(seedSecret)).Should(
			HaveField("Data", HaveKey(kubernetes.KubeConfig)),
			"secret %v should contain the seed kubeconfig",
		)

		clientSet, err := kubernetes.NewClientFromSecretObject(seedSecret,
			kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
			kubernetes.WithDisabledCachedClient(),
		)
		Expect(err).NotTo(HaveOccurred())
		tc.SetSeedClientSet(clientSet)
	}, SpecTimeout(time.Minute))
}

// ItShouldAnnotateSeed sets the given annotation within the seed metadata to the specified value and patches the seed object
func ItShouldAnnotateSeed(tc *SeedContext, annotations map[string]string) {
	GinkgoHelper()

	It("Annotate Seed", func(ctx SpecContext) {
		patch := client.MergeFrom(tc.Seed.DeepCopy())

		for key, value := range annotations {
			tc.Log.Info("Setting annotation", "annotation", key, "value", value)
			metav1.SetMetaDataAnnotation(&tc.Seed.ObjectMeta, key, value)
		}

		Eventually(ctx, func() error {
			return tc.GardenClient.Patch(ctx, tc.Seed, patch)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForSeedToBeReady waits for the seed object to be ready
func ItShouldWaitForSeedToBeReady(tc *SeedContext) {
	GinkgoHelper()

	It("Should wait for seed to be ready", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(tc.Seed), tc.Seed)).To(Succeed())
			g.Expect(health.CheckSeed(tc.Seed, tc.Seed.Status.Gardener)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(10*time.Minute))
}

// ItShouldWaitForSeedToBeDeleted waits for the seed object to be gone
func ItShouldWaitForSeedToBeDeleted(tc *SeedContext) {
	GinkgoHelper()

	It("Wait for Seed to be deleted", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			err := tc.GardenKomega.Get(tc.Seed)()
			if err == nil {
				tc.Log.Info("Waiting for deletion", "lastOperation", tc.Seed.Status.LastOperation)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		tc.Log.Info("Seed has been deleted")
	}, SpecTimeout(10*time.Minute))
}

// ItShouldCreatePrometheusRuleForSeed creates a PrometheusRule and makes sure it is created.
func ItShouldCreatePrometheusRuleForSeed(tc *SeedContext, rule *monitoringv1.PrometheusRule) {
	GinkgoHelper()

	It("Create PrometheusRule "+rule.Namespace+"/"+rule.Name, func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Create(ctx, rule)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldDeletePrometheusRuleForSeed deletes a PrometheusRule and makes sure it is deleted.
func ItShouldDeletePrometheusRuleForSeed(tc *SeedContext, rule *monitoringv1.PrometheusRule) {
	GinkgoHelper()

	It("Delete PrometheusRule "+rule.Namespace+"/"+rule.Name, func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Delete(ctx, rule)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldRenderAndDeployTemplateToSeed renders the given template and deploys it to the given namespace on the seed.
func ItShouldRenderAndDeployTemplateToSeed(tc *SeedContext, namespace, templateName string, values map[string]any) {
	GinkgoHelper()

	It("Render and deploy template to the seed", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			values["HelmDeployNamespace"] = namespace

			return applyManifest(ctx, tc.SeedClientSet, templateName, values)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForPodsInGardenNamespaceToBeReady waits for all pods matching the labels to be ready in the garden namespace on the seed.
func ItShouldWaitForPodsInGardenNamespaceToBeReady(tc *SeedContext, podLabels labels.Selector) {
	GinkgoHelper()

	It("Wait for pods in the seed garden namespace to be ready", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return verifyPodsAreReady(ctx, tc.SeedClientSet, podLabels, "garden")
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

func applyManifest(ctx SpecContext, clientSet kubernetes.Interface, templateName string, params map[string]any) error {
	templatesDirAbs, err := filepath.Abs(seedTemplatesDir)
	if err != nil {
		return err
	}

	templateFilepath := filepath.Join(templatesDirAbs, templateName)
	if _, err := os.Stat(templateFilepath); err != nil {
		return err
	}

	tpl, err := template.
		New(templateName).
		Funcs(sprig.HtmlFuncMap()).
		ParseFiles(templateFilepath)
	if err != nil {
		return err
	}

	var writer bytes.Buffer
	if err = tpl.Execute(&writer, params); err != nil {
		return err
	}

	manifestReader := kubernetes.NewManifestReader(writer.Bytes())
	return clientSet.Applier().ApplyManifest(ctx, manifestReader, kubernetes.DefaultMergeFuncs)
}

func verifyPodsAreReady(ctx SpecContext, clientSet kubernetes.Interface, podLabels labels.Selector, namespace string) error {
	podList := &corev1.PodList{}
	if err := clientSet.Client().List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: podLabels},
	); err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods found in %s with labels %s", namespace, podLabels)
	}

	for _, pod := range podList.Items {
		if !health.IsPodReady(&pod) {
			return fmt.Errorf("pod %s/%s is not running", pod.Namespace, pod.Name)
		}
	}
	return nil
}
