// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	extensionsbackupbucketcontroller "github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	extensionsintegrationtest "github.com/gardener/gardener/test/integration/extensions/controller"
)

const (
	pollInterval        = time.Second
	pollTimeout         = 5 * time.Minute
	pollSevereThreshold = pollTimeout
)

var _ = Describe("BackupBucket", func() {
	It("should successfully create and delete a BackupBucket (ignoring operation annotation)", func() {
		prepareAndRunTest(true)
	})

	It("should successfully create and delete a BackupBucket (respecting operation annotation)", func() {
		prepareAndRunTest(false)
	})
})

var testNamespace *corev1.Namespace

func prepareAndRunTest(ignoreOperationAnnotation bool) {
	By("Create test Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
			GenerateName: testID + "-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

	DeferCleanup(func() {
		By("Delete test Namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	cluster := &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace.Name,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
			Seed:         runtime.RawExtension{Raw: []byte("{}")},
			Shoot:        runtime.RawExtension{Raw: []byte("{}")},
		},
	}

	By("Create Cluster")
	Expect(testClient.Create(ctx, cluster)).To(Succeed())
	log.Info("Created Cluster for test", "cluster", client.ObjectKeyFromObject(cluster))

	DeferCleanup(func() {
		By("Delete Cluster")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, cluster))).To(Succeed())
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  kubernetes.SeedScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{testNamespace.Name: {}},
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	By("Register controller")
	Expect(addTestControllerToManagerWithOptions(mgr, ignoreOperationAnnotation)).To(Succeed())

	By("Start manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	DeferCleanup(func() {
		By("Stop manager")
		mgrCancel()
	})

	runTest(testClient, ignoreOperationAnnotation)
}

func runTest(c client.Client, ignoreOperationAnnotation bool) {
	var (
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: testNamespace.Name,
			},
		}
		secretObjectKey = client.ObjectKeyFromObject(secret)

		backupBucket = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace.Name,
			},
			Spec: extensionsv1alpha1.BackupBucketSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: extensionsintegrationtest.Type,
				},
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: testNamespace.Name,
				},
				Region: "foo",
			},
		}
		backupBucketObjectKey = client.ObjectKeyFromObject(backupBucket)
	)

	By("Create cloudprovider secret")
	Expect(c.Create(ctx, secret)).To(Succeed())

	DeferCleanup(func() {
		By("Delete cloudprovider secret")
		Expect(controllerutils.RemoveFinalizers(ctx, c, secret, extensionsbackupbucketcontroller.FinalizerName)).To(Succeed())
		Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
	})

	By("Create backupbucket")
	timeIn1 := time.Now().String()
	metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, timeIn1)
	Expect(c.Create(ctx, backupBucket)).To(Succeed())

	DeferCleanup(func() {
		By("Delete backupbucket")
		Expect(client.IgnoreNotFound(c.Delete(ctx, backupBucket))).To(Succeed())
	})

	By("Wait until backupbucket is ready")
	Expect(waitForBackupBucketToBeReady(ctx, c, log, backupBucket)).To(Succeed())

	By("Verify secret handling")
	Expect(c.Get(ctx, secretObjectKey, secret)).To(Succeed())
	Expect(secret.Finalizers).To(ConsistOf(extensionsbackupbucketcontroller.FinalizerName))

	By("Verify backupbucket readiness (reconciliation should have happened)")
	backupBucket = &extensionsv1alpha1.BackupBucket{}
	Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
	// When the operation annotation is ignored then there is the secret mapper which may lead to multiple
	// reconciliations, hence we are okay with both Create/Reconcile last operation types.
	// Due to the same reason, the time when the BackupBucket is read it can be under reconciliation.
	// Still, the `Succeeded` state is ensured by each call of waitForBackupBucketToBeReady.
	lastOperationTypeMatcher := Equal(gardencorev1beta1.LastOperationTypeCreate)
	lastOperationStateMatcher := Equal(gardencorev1beta1.LastOperationStateSucceeded)
	if ignoreOperationAnnotation {
		lastOperationTypeMatcher = Or(Equal(gardencorev1beta1.LastOperationTypeCreate), Equal(gardencorev1beta1.LastOperationTypeReconcile))
		lastOperationStateMatcher = Or(Equal(gardencorev1beta1.LastOperationStateSucceeded), Equal(gardencorev1beta1.LastOperationStateProcessing))
	}
	verifyBackupBucket(backupBucket, 1, 1, timeIn1, lastOperationTypeMatcher, lastOperationStateMatcher)

	By("Provoke error in reconciliation")
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)

		// This is to trigger a reconciliation for this error provocation
		backupBucket.Spec.Region += "1"
		if !ignoreOperationAnnotation {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		}
	})).To(Succeed())

	By("Verify backupbucket status transitioned to error")
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(Succeed())
		g.Expect(backupBucket.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
		g.Expect(backupBucket.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
	}).Should(Succeed())

	By("Fix reconciliation error")
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, "")
	})).To(Succeed())

	By("Wait until backupbucket is ready")
	Expect(waitForBackupBucketToBeReady(ctx, c, log, backupBucket)).To(Succeed())

	By("Verify backupbucket (reconciliation should have happened successfully)")
	backupBucket = &extensionsv1alpha1.BackupBucket{}
	Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
	verifyBackupBucket(backupBucket, 2, 2, timeIn1, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

	By("Update time-in annotation (no generation change and no operation annotation -> no reconciliation)")
	timeIn2 := time.Now().String()
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, timeIn2)
	})).To(Succeed())

	By("Verify backupbucket is not reconciled")
	resourceVersion := backupBucket.ResourceVersion
	Consistently(func() string {
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		return backupBucket.ResourceVersion
	}).Should(Equal(resourceVersion))

	By("Verify backupbucket (nothing should have changed)")
	backupBucket = &extensionsv1alpha1.BackupBucket{}
	Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
	verifyBackupBucket(backupBucket, 2, 2, timeIn1, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

	if ignoreOperationAnnotation {
		By("Update backupbucket spec (generation change -> reconciliation)")
		timeIn3 := time.Now().String()
		Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, timeIn3)
			backupBucket.Spec.Region += "1"
		})).To(Succeed())

		By("Wait until backupbucket is ready")
		Expect(waitForBackupBucketToBeReady(ctx, c, log, backupBucket)).To(Succeed())

		By("Verify backupbucket readiness (reconciliation should have happened)")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 3, timeIn3, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

		By("Update time-in annotation (to test secret mapping)")
		timeIn4 := time.Now().String()
		Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, timeIn4)
		})).To(Succeed())

		By("Wait until backupbucket is ready")
		Expect(waitForBackupBucketToBeReady(ctx, c, log, backupBucket)).To(Succeed())

		By("Verify backupbucket readiness")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 3, timeIn3, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

		By("Generate secret event")
		metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "foo", "bar")
		Expect(c.Update(ctx, secret)).To(Succeed())

		By("Wait until backupbucket is ready")
		// also wait for lastOperation's update time to be updated to give extension controller some time to observe
		// secret event and start reconciliation
		Expect(waitForBackupBucketToBeReady(ctx, c, log, backupBucket, backupBucket.Status.LastOperation.LastUpdateTime)).To(Succeed())

		By("Verify backupbucket readiness (reconciliation should have happened due to secret mapping)")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 3, timeIn4, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)
	} else {
		By("Update backupbucket spec (generation change but no operation annotation -> no reconciliation)")
		timeIn3 := time.Now().String()
		Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, timeIn3)
			backupBucket.Spec.Region += "1"
		})).To(Succeed())

		By("Verify backupbucket (nothing should have changed)")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 2, timeIn1, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

		By("Add operation annotation (should trigger reconciliation)")
		Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		})).To(Succeed())

		By("Wait until backupbucket is ready")
		Expect(waitForBackupBucketToBeReady(ctx, c, log, backupBucket)).To(Succeed())

		By("Verify backupbucket (reconciliation should have happened due to operation annotation)")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 3, timeIn3, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)
		Expect(backupBucket.Annotations).ToNot(HaveKey(v1beta1constants.GardenerOperation))
	}

	By("Provoke error in deletion")
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
	})).To(Succeed())

	By("Delete backupbucket")
	Expect(client.IgnoreNotFound(c.Delete(ctx, backupBucket))).To(Succeed())

	By("Verify backupbucket status transitioned to error")
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(Succeed())
		g.Expect(backupBucket.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeDelete))
		g.Expect(backupBucket.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
	}).Should(Succeed())

	By("Fix deletion error")
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, "")
	})).To(Succeed())

	By("Wait until backupbucket is deleted")
	Expect(waitForBackupBucketToBeDeleted(ctx, log, backupBucket, c)).NotTo(HaveOccurred())

	By("Verify deletion of backupbucket")
	Expect(c.Get(ctx, client.ObjectKey{Name: testNamespace.Name}, testNamespace)).To(Succeed())
	Expect(testNamespace.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperation]).To(Equal(extensionsintegrationtest.AnnotationValueOperationDelete))

	By("Verify if finalizer has been released from secret")
	secret = &corev1.Secret{}
	Expect(c.Get(ctx, secretObjectKey, secret)).To(Succeed())
	Expect(secret.Finalizers).NotTo(ConsistOf(extensionsbackupbucketcontroller.FinalizerName))
}

func patchBackupBucketObject(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket, transform func()) error {
	patch := client.MergeFrom(backupBucket.DeepCopy())
	transform()
	return c.Patch(ctx, backupBucket, patch)
}

func waitForBackupBucketToBeReady(ctx context.Context, c client.Client, log logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket, minOperationUpdateTime ...metav1.Time) error {
	healthFuncs := []health.Func{health.CheckExtensionObject}
	if len(minOperationUpdateTime) > 0 {
		healthFuncs = append(healthFuncs, health.ExtensionOperationHasBeenUpdatedSince(minOperationUpdateTime[0]))
	}

	return extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		c,
		log,
		health.And(healthFuncs...),
		backupBucket,
		extensionsv1alpha1.BackupBucketResource,
		pollInterval,
		pollSevereThreshold,
		pollTimeout,
		nil,
	)
}

func waitForBackupBucketToBeDeleted(ctx context.Context, log logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket, c client.Client) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c,
		log,
		backupBucket,
		extensionsv1alpha1.BackupBucketResource,
		pollInterval,
		pollTimeout,
	)
}

func verifyBackupBucket(backupBucket *extensionsv1alpha1.BackupBucket, generation, observedGeneration int64, expectedTimeOut string, expectedLastOperationType, expectedLastOperationState gomegatypes.GomegaMatcher) {
	ExpectWithOffset(1, backupBucket.Generation).To(Equal(generation))
	ExpectWithOffset(1, backupBucket.Finalizers).To(ConsistOf(extensionsbackupbucketcontroller.FinalizerName))
	ExpectWithOffset(1, backupBucket.Status.LastOperation.Type).To(expectedLastOperationType)
	ExpectWithOffset(1, backupBucket.Status.LastOperation.State).To(expectedLastOperationState)
	ExpectWithOffset(1, backupBucket.Status.ObservedGeneration).To(Equal(observedGeneration))
	ExpectWithOffset(1, backupBucket.Annotations[extensionsintegrationtest.AnnotationKeyTimeOut]).To(Equal(expectedTimeOut))
}
