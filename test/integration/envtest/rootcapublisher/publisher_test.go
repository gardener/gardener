package rootcapublisher_test

import (
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Root CA Controller tests", func() {
	var (
		namespace *corev1.Namespace
		configMap *corev1.ConfigMap
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-root-ca.crt",
				Namespace: namespace.Name,
			},
		}

	})

	// Open:
	// Update on namespace -> create secret

	//
	It("should successfully create a config map on creating a namespace", func() {
		Expect(testClient.Create(ctx, namespace)).To(Or(Succeed(), BeAlreadyExistsError()))

		// TODO possibly reduce timeout
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
		}).Should(Succeed())
	})

	It("should keep the secret in the desired state after Delete/Update of the secret", func() {
		Expect(testClient.Create(ctx, namespace)).To(Or(Succeed(), BeAlreadyExistsError()))
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
		}).Should(Succeed())

		By("Deleting the secret")
		Expect(testClient.Delete(ctx, configMap)).To(Succeed())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
		}).Should(BeNotFoundError())

		// TODO possibly reduce timeout
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
		}).Should(Succeed())

		By("Updating the secret")
		configMap.Data = nil
		Expect(testClient.Update(ctx, configMap)).To(Succeed())

		// TODO possibly reduce timeout
		Eventually(func() bool {
			testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			return configMap.Data != nil
		}).Should(BeTrue())

		// create a secret with a different name
		By("Ignoring annotating configmap")
		configMap.Data = nil
		configMap.Annotations = map[string]string{"kubernetes.io/description": "test description"}
		Expect(testClient.Update(ctx, configMap)).To(Succeed())

		// TODO possibly reduce timeout
		Consistently(func() bool {
			testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			return configMap.Data == nil
		}).Should(BeTrue())

		By("Ignoring configmap with different name")
		// Create cm with different name
		// update cm
		// check that data is nil
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "secret",
				Namespace:   namespace.Name,
				Annotations: map[string]string{"foo": "bar"},
			},
		}
		Expect(testClient.Create(ctx, cm)).To(Succeed())

		baseCM := cm.DeepCopyObject().(client.Object)
		cm.Annotations["foo"] = "newbar"
		Expect(testClient.Patch(ctx, cm, client.MergeFrom(baseCM))).To(Succeed())

		Expect(cm.Annotations).To(HaveLen(1))
		Expect(cm.Annotations).To(HaveKeyWithValue("foo", "newbar"))
	})
})
