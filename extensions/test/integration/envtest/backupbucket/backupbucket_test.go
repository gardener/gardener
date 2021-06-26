// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupbucket

import (
	"context"
	"fmt"
	"time"

	backupbucketcontroller "github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/extensions/test/integration"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	pollInterval        = time.Second
	pollTimeout         = 5 * time.Minute
	pollSevereThreshold = pollTimeout
)

var _ = Describe("BackupBucket", func() {
	AfterEach(func() {
		framework.RunCleanupActions()
	})

	It("should successfully create and delete a BackupBucket (ignoring operation annotation)", func() {
		prepareAndRunTest(true)
	})

	It("should successfully create and delete a BackupBucket (respecting operation annotation)", func() {
		prepareAndRunTest(false)
	})
})

func prepareAndRunTest(ignoreOperationAnnotation bool) {
	By("setup and start manager")
	Expect(createAndStartManager(ignoreOperationAnnotation)).To(Succeed())

	By("setup client for test")
	// We could also get the manager client with mgr.GetClient(), however, this one would use a cache in the background
	// which may lead to outdated results when using it later on. Hence, we create a dedicated client without a cache
	// so that the test always reads the most up-to-date state of a resource.
	c, err := client.New(restConfig, client.Options{Scheme: kubernetes.SeedScheme})
	Expect(err).NotTo(HaveOccurred())

	By("generate namespace name for test")
	namespace, err := generateNamespaceName()
	Expect(err).NotTo(HaveOccurred())

	runTest(c, namespace, ignoreOperationAnnotation)
}

func createAndStartManager(ignoreOperationAnnotation bool) error {
	mgrContext, mgrCancel := context.WithCancel(ctx)

	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {
		defer func() {
			By("stopping manager")
			mgrCancel()
		}()

		framework.RemoveCleanupAction(cleanupHandle)
	})

	mgrScheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(scheme.AddToScheme, extensionsv1alpha1.AddToScheme)
	if err := schemeBuilder.AddToScheme(mgrScheme); err != nil {
		return err
	}

	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             mgrScheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		return err
	}

	if err := addTestControllerToManagerWithOptions(mgr, ignoreOperationAnnotation); err != nil {
		return err
	}

	go func() {
		defer GinkgoRecover()
		if err := mgr.Start(mgrContext); err != nil {
			Expect(err).NotTo(HaveOccurred())
		}
	}()

	return nil
}

func generateNamespaceName() (string, error) {
	suffix, err := utils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}
	return "gextlib-backupbucket-test--" + suffix, nil
}

func runTest(c client.Client, namespaceName string, ignoreOperationAnnotation bool) {
	var (
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
				Seed:         runtime.RawExtension{Raw: []byte("{}")},
				Shoot:        runtime.RawExtension{Raw: []byte("{}")},
			},
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespaceName,
			},
		}
		secretObjectKey = client.ObjectKey{Namespace: secret.Namespace, Name: secret.Name}

		backupBucket = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
			Spec: extensionsv1alpha1.BackupBucketSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: integration.Type,
				},
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespaceName,
				},
				Region: "foo",
			},
		}
		backupBucketObjectKey = client.ObjectKey{Name: namespaceName}
	)

	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {
		if backupBucket.Name != "" {
			By("delete backupbucket")
			Expect(client.IgnoreNotFound(c.Delete(ctx, backupBucket))).To(Succeed())
		}

		By("delete secret")
		Expect(controllerutils.RemoveFinalizer(ctx, c, c, secret, backupbucketcontroller.FinalizerName)).To(Succeed())
		Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())

		By("delete cluster")
		Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())

		By("delete namespace")
		Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())

		framework.RemoveCleanupAction(cleanupHandle)
	})

	By("create namespace for test execution")
	Expect(c.Create(ctx, namespace)).To(Succeed())

	By("create cluster")
	Expect(c.Create(ctx, cluster)).To(Succeed())

	By("create cloudprovider secret into namespace")
	Expect(c.Create(ctx, secret)).To(Succeed())

	By("create backupbucket")
	timeIn1 := time.Now().String()
	metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, integration.AnnotationKeyTimeIn, timeIn1)
	Expect(c.Create(ctx, backupBucket)).To(Succeed())

	By("wait until backupbucket is ready")
	Expect(waitForBackupBucketToBeReady(ctx, c, logger, backupBucket)).To(Succeed())

	By("verify secret handling")
	Expect(c.Get(ctx, secretObjectKey, secret)).To(Succeed())
	Expect(secret.Finalizers).To(ConsistOf(backupbucketcontroller.FinalizerName))

	By("verify backupbucket readiness (reconciliation should have happened)")
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

	By("provoke error in reconciliation")
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, integration.AnnotationKeyDesiredOperationState, integration.AnnotationValueDesiredOperationStateError)

		// This is to trigger a reconciliation for this error provocation
		backupBucket.Spec.Region += "1"
		if !ignoreOperationAnnotation {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		}
	})).To(Succeed())

	By("verify backupbucket status transitioned to error")
	Expect(waitForBackupBucketToBeErroneous(ctx, c, gardencorev1beta1.LastOperationTypeReconcile, backupBucketObjectKey)).To(Succeed())

	By("fixing reconciliation error")
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, integration.AnnotationKeyDesiredOperationState, "")
	})).To(Succeed())

	By("wait until backupbucket is ready")
	Expect(waitForBackupBucketToBeReady(ctx, c, logger, backupBucket)).To(Succeed())

	By("verify backupbucket (reconciliation should have happened successfully)")
	backupBucket = &extensionsv1alpha1.BackupBucket{}
	Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
	verifyBackupBucket(backupBucket, 2, 2, timeIn1, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

	By("update time-in annotation (no generation change and no operation annotation -> no reconciliation)")
	timeIn2 := time.Now().String()
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, integration.AnnotationKeyTimeIn, timeIn2)
	})).To(Succeed())

	By("verify backupbucket is not reconciled")
	resourceVersion := backupBucket.ResourceVersion
	Consistently(func() string {
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		return backupBucket.ResourceVersion
	}, 2, 0.1).Should(Equal(resourceVersion))

	By("verify backupbucket (nothing should have changed)")
	backupBucket = &extensionsv1alpha1.BackupBucket{}
	Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
	verifyBackupBucket(backupBucket, 2, 2, timeIn1, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

	if ignoreOperationAnnotation {
		By("update backupbucket spec (generation change -> reconciliation)")
		timeIn3 := time.Now().String()
		Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, integration.AnnotationKeyTimeIn, timeIn3)
			backupBucket.Spec.Region += "1"
		})).To(Succeed())

		By("wait until backupbucket is ready")
		Expect(waitForBackupBucketToBeReady(ctx, c, logger, backupBucket)).To(Succeed())

		By("verify backupbucket readiness (reconciliation should have happened)")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 3, timeIn3, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

		By("update time-in annotation (to test secret mapping)")
		timeIn4 := time.Now().String()
		Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, integration.AnnotationKeyTimeIn, timeIn4)
		})).To(Succeed())

		By("wait until backupbucket is ready")
		Expect(waitForBackupBucketToBeReady(ctx, c, logger, backupBucket)).To(Succeed())

		By("verify backupbucket readiness")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 3, timeIn3, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

		By("generate secret event")
		metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "foo", "bar")
		Expect(c.Update(ctx, secret)).To(Succeed())

		By("wait until backupbucket is ready")
		// also wait for lastOperation's update time to be updated to give extension controller some time to observe
		// secret event and start reconciliation
		Expect(waitForBackupBucketToBeReady(ctx, c, logger, backupBucket, backupBucket.Status.LastOperation.LastUpdateTime)).To(Succeed())

		By("verify backupbucket readiness (reconciliation should have happened due to secret mapping)")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 3, timeIn4, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)
	} else {
		By("update backupbucket spec (generation change but no operation annotation -> no reconciliation)")
		timeIn3 := time.Now().String()
		Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, integration.AnnotationKeyTimeIn, timeIn3)
			backupBucket.Spec.Region += "1"
		})).To(Succeed())

		By("verify backupbucket (nothing should have changed)")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 2, timeIn1, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)

		By("add operation annotation (should trigger reconciliation)")
		Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
			metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		})).To(Succeed())

		By("wait until backupbucket is ready")
		Expect(waitForBackupBucketToBeReady(ctx, c, logger, backupBucket)).To(Succeed())

		By("verify backupbucket (reconciliation should have happened due to operation annotation)")
		backupBucket = &extensionsv1alpha1.BackupBucket{}
		Expect(c.Get(ctx, backupBucketObjectKey, backupBucket)).To(Succeed())
		verifyBackupBucket(backupBucket, 3, 3, timeIn3, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)
		Expect(backupBucket.Annotations).ToNot(HaveKey(v1beta1constants.GardenerOperation))
	}

	By("provoke error in deletion")
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, integration.AnnotationKeyDesiredOperationState, integration.AnnotationValueDesiredOperationStateError)
	})).To(Succeed())

	By("delete backupbucket")
	Expect(client.IgnoreNotFound(c.Delete(ctx, backupBucket))).To(Succeed())

	By("verify backupbucket status transitioned to error")
	Expect(waitForBackupBucketToBeErroneous(ctx, c, gardencorev1beta1.LastOperationTypeDelete, backupBucketObjectKey)).To(Succeed())

	By("fixing deletion error")
	Expect(patchBackupBucketObject(ctx, c, backupBucket, func() {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, integration.AnnotationKeyDesiredOperationState, "")
	})).To(Succeed())

	By("wait until backupbucket is deleted")
	Expect(waitForBackupBucketToBeDeleted(ctx, c, logger, backupBucket)).NotTo(HaveOccurred())

	By("verify deletion of backupbucket")
	Expect(c.Get(ctx, client.ObjectKey{Name: namespaceName}, namespace)).To(Succeed())
	Expect(namespace.Annotations[integration.AnnotationKeyDesiredOperation]).To(Equal(integration.AnnotationValueOperationDelete))

	By("check if finalizer has been released from secret")
	secret = &corev1.Secret{}
	Expect(c.Get(ctx, secretObjectKey, secret)).To(Succeed())
	Expect(secret.Finalizers).NotTo(ConsistOf(backupbucketcontroller.FinalizerName))
}

func patchBackupBucketObject(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket, transform func()) error {
	patch := client.MergeFrom(backupBucket.DeepCopy())
	transform()
	return c.Patch(ctx, backupBucket, patch)
}

func waitForBackupBucketToBeReady(ctx context.Context, c client.Client, logger *logrus.Entry, backupBucket *extensionsv1alpha1.BackupBucket, minOperationUpdateTime ...metav1.Time) error {
	healthFuncs := []health.Func{health.CheckExtensionObject}
	if len(minOperationUpdateTime) > 0 {
		healthFuncs = append(healthFuncs, health.ExtensionOperationHasBeenUpdatedSince(minOperationUpdateTime[0]))
	}

	return extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		c,
		logger,
		health.And(healthFuncs...),
		backupBucket,
		extensionsv1alpha1.BackupBucketResource,
		pollInterval,
		pollSevereThreshold,
		pollTimeout,
		nil,
	)
}

func waitForBackupBucketToBeDeleted(ctx context.Context, c client.Client, logger *logrus.Entry, backupBucket *extensionsv1alpha1.BackupBucket) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c,
		logger,
		backupBucket,
		extensionsv1alpha1.BackupBucketResource,
		pollInterval,
		pollTimeout,
	)
}

func waitForBackupBucketToBeErroneous(ctx context.Context, c client.Client, lastOperationType gardencorev1beta1.LastOperationType, backupBucketObjectKey client.ObjectKey) error {
	return retryutils.UntilTimeout(ctx, pollInterval, pollTimeout, func(ctx context.Context) (bool, error) {
		backupBucket := &extensionsv1alpha1.BackupBucket{}
		if err := c.Get(ctx, backupBucketObjectKey, backupBucket); err != nil {
			return retryutils.SevereError(err)
		}

		if backupBucket.Status.LastOperation.Type == lastOperationType &&
			backupBucket.Status.LastOperation.State == gardencorev1beta1.LastOperationStateError {
			return retryutils.Ok()
		}

		return retryutils.MinorError(fmt.Errorf("waiting for backupbucket to be erroneous (current state is %q)", backupBucket.Status.LastOperation.State))
	})
}

func verifyBackupBucket(backupBucket *extensionsv1alpha1.BackupBucket, generation, observedGeneration int64, expectedTimeOut string, expectedLastOperationType, expectedLastOperationState gomegatypes.GomegaMatcher) {
	ExpectWithOffset(1, backupBucket.Generation).To(Equal(generation))
	ExpectWithOffset(1, backupBucket.Finalizers).To(ConsistOf(backupbucketcontroller.FinalizerName))
	ExpectWithOffset(1, backupBucket.Status.LastOperation.Type).To(expectedLastOperationType)
	ExpectWithOffset(1, backupBucket.Status.LastOperation.State).To(expectedLastOperationState)
	ExpectWithOffset(1, backupBucket.Status.ObservedGeneration).To(Equal(observedGeneration))
	ExpectWithOffset(1, backupBucket.Annotations[integration.AnnotationKeyTimeOut]).To(Equal(expectedTimeOut))
}
