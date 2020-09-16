// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"fmt"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultTimeout         = 30 * time.Second
	defaultInterval        = 5 * time.Second
	defaultSevereThreshold = 15 * time.Second
)

// Actuator acts upon BackupBucket resources.
type Actuator interface {
	// Reconcile reconciles the BackupBucket.
	Reconcile(context.Context) error
	// Delete deletes the BackupBucket.
	Delete(context.Context) error
}

type actuator struct {
	gardenClient kubernetes.Interface
	seedClient   kubernetes.Interface
	logger       *logrus.Entry
	backupBucket *gardencorev1beta1.BackupBucket
}

func newActuator(gardenClient, seedClient kubernetes.Interface, bb *gardencorev1beta1.BackupBucket, logger logrus.FieldLogger) Actuator {
	return &actuator{
		logger:       logger.WithField("backupbucket", bb.Name),
		backupBucket: bb,
		gardenClient: gardenClient,
		seedClient:   seedClient,
	}
}

func (a *actuator) Reconcile(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup Bucket Reconciliation")

		deployBackupBucketExtension = g.Add(flow.Task{
			Name: "Deploying backup bucket extension resource",
			Fn:   flow.TaskFn(a.deployBackupBucketExtension).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})

		_ = g.Add(flow.Task{
			Name:         "Waiting until backup bucket is reconciled",
			Fn:           flow.TaskFn(a.waitUntilBackupBucketExtensionReconciled),
			Dependencies: flow.NewTaskIDs(deployBackupBucketExtension),
		})
		f = g.Compile()
	)

	return f.Run(flow.Opts{
		Logger:           a.logger,
		ProgressReporter: a.reportBackupBucketProgress,
		Context:          ctx,
	})
}

func (a *actuator) Delete(ctx context.Context) error {
	var (
		g                  = flow.NewGraph("Backup bucket deletion")
		deleteBackupBucket = g.Add(flow.Task{
			Name: "Destroying backup bucket",
			Fn:   flow.TaskFn(a.deleteBackupBucketExtension),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension backup bucket is deleted",
			Fn:           flow.TaskFn(a.waitUntilBackupBucketExtensionDeleted),
			Dependencies: flow.NewTaskIDs(deleteBackupBucket),
		})
		f = g.Compile()
	)
	return f.Run(flow.Opts{
		Logger:           a.logger,
		ProgressReporter: a.reportBackupBucketProgress,
		Context:          ctx,
	})
}

// reportBackupBucketProgress will update the phase and error in the BackupBucket manifest `status` section
// by the current progress of the Flow execution.
func (a *actuator) reportBackupBucketProgress(ctx context.Context, stats *flow.Stats) {
	if err := kutil.TryUpdateStatus(ctx, kretry.DefaultBackoff, a.gardenClient.DirectClient(), a.backupBucket, func() error {
		if a.backupBucket.Status.LastOperation == nil {
			return fmt.Errorf("last operation of BackupBucket %s/%s is unset", a.backupBucket.Namespace, a.backupBucket.Name)
		}
		a.backupBucket.Status.LastOperation.Description = makeDescription(stats)
		a.backupBucket.Status.LastOperation.Progress = stats.ProgressPercent()
		a.backupBucket.Status.LastOperation.LastUpdateTime = metav1.Now()
		return nil
	}); err != nil {
		a.logger.Warnf("could not report backupbucket progress with description: %s: %v", makeDescription(stats), err)
	}
}

func makeDescription(stats *flow.Stats) string {
	if stats.ProgressPercent() == 100 {
		return "Execution finished"
	}
	return strings.Join(stats.Running.StringList(), ", ")
}

// deployBackupBucketExtension deploys the BackupBucket extension resource in Seed with the required secret.
func (a *actuator) deployBackupBucketExtension(ctx context.Context) error {
	coreSecret, err := common.GetSecretFromSecretRef(ctx, a.gardenClient.Client(), &a.backupBucket.Spec.SecretRef)
	if err != nil {
		return err
	}

	extensionSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupBucketSecretName(a.backupBucket.Name),
			Namespace: v1beta1constants.GardenNamespace,
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, a.seedClient.Client(), extensionSecret, func() error {
		extensionSecret.Data = coreSecret.DeepCopy().Data
		return nil
	}); err != nil {
		return err
	}

	// create extension backup bucket resource in seed
	extensionBackupBucket := &extensionsv1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.backupBucket.Name,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, a.seedClient.Client(), extensionBackupBucket, func() error {
		metav1.SetMetaDataAnnotation(&extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().String())

		extensionBackupBucket.Spec = extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           a.backupBucket.Spec.Provider.Type,
				ProviderConfig: a.backupBucket.Spec.ProviderConfig,
			},
			Region: a.backupBucket.Spec.Provider.Region,
			SecretRef: corev1.SecretReference{
				Name:      generateBackupBucketSecretName(a.backupBucket.Name),
				Namespace: v1beta1constants.GardenNamespace,
			},
		}
		return nil
	})
	return err
}

// waitUntilBackupBucketExtensionReconciled waits until BackupBucket Extension resource reconciled from seed.
// It also copies the generatedSecret from seed to garden.
func (a *actuator) waitUntilBackupBucketExtensionReconciled(ctx context.Context) error {
	return common.WaitUntilExtensionCRReady(
		ctx,
		a.seedClient.DirectClient(),
		a.logger,
		func() runtime.Object { return &extensionsv1alpha1.BackupBucket{} },
		"BackupBucket",
		"",
		a.backupBucket.Name,
		defaultInterval,
		defaultSevereThreshold,
		defaultTimeout,
		func(obj runtime.Object) error {
			backupBucket, ok := obj.(*extensionsv1alpha1.BackupBucket)
			if !ok {
				return fmt.Errorf("expected extensionsv1alpha1.BackupBucket but got %T", backupBucket)
			}

			var generatedSecretRef *corev1.SecretReference

			if backupBucket.Status.GeneratedSecretRef != nil {
				generatedSecret, err := common.GetSecretFromSecretRef(ctx, a.seedClient.Client(), backupBucket.Status.GeneratedSecretRef)
				if err != nil {
					return err
				}

				coreGeneratedSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      generateGeneratedBackupBucketSecretName(a.backupBucket.Name),
						Namespace: v1beta1constants.GardenNamespace,
					},
				}
				ownerRef := metav1.NewControllerRef(backupBucket, gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket"))

				if _, err := controllerutil.CreateOrUpdate(ctx, a.gardenClient.Client(), coreGeneratedSecret, func() error {
					coreGeneratedSecret.OwnerReferences = []metav1.OwnerReference{*ownerRef}
					coreGeneratedSecret.Data = generatedSecret.DeepCopy().Data

					finalizers := sets.NewString(coreGeneratedSecret.GetFinalizers()...)
					finalizers.Insert(finalizerName)
					coreGeneratedSecret.SetFinalizers(finalizers.UnsortedList())

					return nil
				}); err != nil {
					return err
				}

				generatedSecretRef = &corev1.SecretReference{
					Name:      coreGeneratedSecret.Name,
					Namespace: coreGeneratedSecret.Namespace,
				}
			}

			if generatedSecretRef != nil || backupBucket.Status.ProviderStatus != nil {
				_, err := controllerutil.CreateOrUpdate(ctx, a.gardenClient.Client(), a.backupBucket, func() error {
					a.backupBucket.Status.GeneratedSecretRef = generatedSecretRef
					a.backupBucket.Status.ProviderStatus = backupBucket.Status.ProviderStatus
					return nil
				})
				return err
			}

			return nil
		},
	)
}

// deleteBackupBucketExtension deletes BackupBucket extension resource in seed.
func (a *actuator) deleteBackupBucketExtension(ctx context.Context) error {
	if err := a.deleteGeneratedBackupBucketSecretInGarden(ctx); err != nil {
		return err
	}

	if err := a.deleteBackupBucketExtensionSecret(ctx); err != nil {
		return err
	}

	return common.DeleteExtensionCR(
		ctx,
		a.seedClient.DirectClient(),
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupBucket{} },
		"",
		a.backupBucket.Name,
	)
}

// waitUntilBackupBucketExtensionDeleted waits until backup bucket extension resource is deleted in seed cluster.
func (a *actuator) waitUntilBackupBucketExtensionDeleted(ctx context.Context) error {
	return common.WaitUntilExtensionCRDeleted(
		ctx,
		a.seedClient.DirectClient(),
		a.logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupBucket{} },
		"BackupBucket",
		"",
		a.backupBucket.Name,
		defaultInterval,
		defaultTimeout,
	)
}

// deleteBackupBucketExtensionSecret deletes secret referred by BackupBucket extension resource in seed.
func (a *actuator) deleteBackupBucketExtensionSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupBucketSecretName(a.backupBucket.Name),
			Namespace: v1beta1constants.GardenNamespace,
		},
	}

	return client.IgnoreNotFound(a.seedClient.Client().Delete(ctx, secret))
}

// deleteGeneratedBackupBucketSecretInGarden deletes generated secret referred by core BackupBucket resource in garden.
func (a *actuator) deleteGeneratedBackupBucketSecretInGarden(ctx context.Context) error {
	if a.backupBucket.Status.GeneratedSecretRef == nil {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.backupBucket.Status.GeneratedSecretRef.Name,
			Namespace: a.backupBucket.Status.GeneratedSecretRef.Namespace,
		},
	}

	if err := controllerutils.RemoveFinalizer(ctx, a.gardenClient.DirectClient(), secret, finalizerName); err != nil {
		return err
	}
	return client.IgnoreNotFound(a.gardenClient.Client().Delete(ctx, secret))
}

func generateBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("bucket-%s", backupBucketName)
}

func generateGeneratedBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("generated-bucket-%s", backupBucketName)
}
