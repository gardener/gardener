// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/extension"
	"github.com/gardener/gardener/pkg/logger"
)

type errorClient struct {
	client.Client
	err error
}

func (e *errorClient) Delete(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
	return e.err
}

var (
	objectIdentifier = Identifier(func(obj any) string {
		switch o := obj.(type) {
		case extensionsv1alpha1.Extension:
			return o.GetName()
		}
		return obj.(client.Object).GetName()
	})
	alwaysMatch = And()
)

func consistOfObjects(names ...string) gomegatypes.GomegaMatcher {
	elements := make(Elements, len(names))
	for _, name := range names {
		elements[name] = alwaysMatch
	}

	return MatchAllElements(objectIdentifier, elements)
}

var _ = Describe("Extension", func() {
	const (
		defaultName     = "def"
		afterName       = "after"
		beforeName      = "before"
		afterWorkerName = "after-worker"
	)

	var (
		fakeSeedClient client.Client
		namespace      *corev1.Namespace
		ctx            = context.TODO()
		ext            extension.Interface
		log            logr.Logger

		defaultExtension     *extensionsv1alpha1.Extension
		beforeExtension      *extensionsv1alpha1.Extension
		afterExtension       *extensionsv1alpha1.Extension
		afterWorkerExtension *extensionsv1alpha1.Extension
		allExtensions        []*extensionsv1alpha1.Extension

		requiredExtensions map[string]extension.Extension
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"}}

		logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
		log = logf.Log.WithName("extensions")

		defaultExtension = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaultName,
				Namespace: namespace.Name,
			},
			Spec: extensionsv1alpha1.ExtensionSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: defaultName,
				},
			},
		}

		beforeKubeAPIServer := gardencorev1beta1.BeforeKubeAPIServer
		beforeExtension = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      beforeName,
				Namespace: namespace.Name,
			},
			Spec: extensionsv1alpha1.ExtensionSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: beforeName,
				},
			},
		}

		afterKubeAPIServer := gardencorev1beta1.AfterKubeAPIServer
		afterExtension = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      afterName,
				Namespace: namespace.Name,
			},
			Spec: extensionsv1alpha1.ExtensionSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: afterName,
				},
			},
		}

		afterWorker := gardencorev1beta1.AfterWorker
		afterWorkerExtension = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      afterWorkerName,
				Namespace: namespace.Name,
			},
			Spec: extensionsv1alpha1.ExtensionSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: afterWorkerName,
				},
			},
		}

		allExtensions = []*extensionsv1alpha1.Extension{defaultExtension, beforeExtension, afterExtension, afterWorkerExtension}

		requiredExtensions = map[string]extension.Extension{
			defaultName: {
				Extension: *defaultExtension,
				Timeout:   time.Second,
			},
			beforeName: {
				Extension: *beforeExtension,
				Timeout:   time.Second,
				Lifecycle: &gardencorev1beta1.ControllerResourceLifecycle{
					Reconcile: &beforeKubeAPIServer,
					Delete:    &beforeKubeAPIServer,
					Migrate:   &beforeKubeAPIServer,
				},
			},
			afterName: {
				Extension: *afterExtension,
				Timeout:   time.Second,
				Lifecycle: &gardencorev1beta1.ControllerResourceLifecycle{
					Reconcile: &afterKubeAPIServer,
					Delete:    &afterKubeAPIServer,
					Migrate:   &afterKubeAPIServer,
				},
			},
			afterWorkerName: {
				Extension: *afterWorkerExtension,
				Timeout:   time.Second,
				Lifecycle: &gardencorev1beta1.ControllerResourceLifecycle{
					Reconcile: &afterWorker,
				},
			},
		}

		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithStatusSubresource(&extensionsv1alpha1.Extension{}).Build()

		ext = extension.New(
			log,
			fakeSeedClient,
			&extension.Values{
				Namespace:  namespace.Name,
				Extensions: requiredExtensions,
			},
			time.Microsecond*100,
			time.Microsecond*400,
			time.Second,
		)
	})

	Describe("#DeployBeforeKubeAPIServer", func() {
		It("should successfully deploy extension resources", func() {
			Expect(ext.DeployBeforeKubeAPIServer(ctx)).To(Succeed())
			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(fakeSeedClient.List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(beforeName))
		})
	})

	Describe("#DeployAfterKubeAPIServer", func() {
		It("should successfully deploy extension resources", func() {
			Expect(ext.DeployAfterKubeAPIServer(ctx)).To(Succeed())
			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(fakeSeedClient.List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(defaultName, afterName))
		})
	})

	Describe("#DeployAfterWorker", func() {
		It("should successfully deploy extension resources", func() {
			Expect(ext.DeployAfterWorker(ctx)).To(Succeed())
			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(fakeSeedClient.List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(afterWorkerName))
		})
	})

	Describe("#WaitBeforeKubeAPIServer", func() {
		It("should return error when no resources are found", func() {
			Expect(ext.WaitBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should return error when resource is not ready", func() {
			errDescription := "Some error"
			beforeExtension.Status = extensionsv1alpha1.ExtensionStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastError: &gardencorev1beta1.LastError{
						Description: errDescription,
					},
				},
			}
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(ext.WaitBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("Error while waiting for Extension test-namespace/before to become ready: error during reconciliation: "+errDescription)), "extensions indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			now := time.Now()
			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(ext.DeployBeforeKubeAPIServer(ctx)).To(Succeed())

			By("Patch object")
			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(beforeExtension), beforeExtension)).To(Succeed())
			patch := client.MergeFrom(beforeExtension.DeepCopy())
			// remove operation annotation, add old timestamp annotation
			beforeExtension.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
			}
			// set last operation
			beforeExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(fakeSeedClient.Patch(ctx, beforeExtension, patch)).ToNot(HaveOccurred(), "patching extension succeeds")

			By("Wait")
			Expect(ext.WaitBeforeKubeAPIServer(ctx)).NotTo(Succeed())
		})
	})

	Describe("#WaitAfterKubeAPIServer", func() {
		It("should return error when no resources are found", func() {
			Expect(ext.WaitAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should return error when resource is not ready", func() {
			errDescription := "Some error"
			defaultExtension.Status = extensionsv1alpha1.ExtensionStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastError: &gardencorev1beta1.LastError{
						Description: errDescription,
					},
				},
			}
			afterExtension.Status = extensionsv1alpha1.ExtensionStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastError: &gardencorev1beta1.LastError{
						Description: errDescription,
					},
				},
			}
			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(ext.WaitAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("Error while waiting for Extension test-namespace/def to become ready: error during reconciliation: "+errDescription)), "extensions indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			now := time.Now()
			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(ext.DeployAfterKubeAPIServer(ctx)).To(Succeed())

			By("Patch object")
			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(defaultExtension), defaultExtension)).To(Succeed())
			patch := client.MergeFrom(defaultExtension.DeepCopy())
			// remove operation annotation, add old timestamp annotation
			defaultExtension.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
			}
			// set last operation
			defaultExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(fakeSeedClient.Patch(ctx, defaultExtension, patch)).ToNot(HaveOccurred(), "patching extension succeeds")

			By("Wait")
			Expect(ext.WaitAfterKubeAPIServer(ctx)).NotTo(Succeed())
		})
	})

	Describe("#WaitAfterWorker", func() {
		It("should return error when no resources are found", func() {
			Expect(ext.WaitAfterWorker(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should return error when resource is not ready", func() {
			errDescription := "Some error"
			afterWorkerExtension.Status = extensionsv1alpha1.ExtensionStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastError: &gardencorev1beta1.LastError{
						Description: errDescription,
					},
				},
			}
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, afterWorkerExtension)).To(Succeed())
			Expect(ext.WaitAfterWorker(ctx)).To(MatchError(ContainSubstring("Error while waiting for Extension test-namespace/after-worker to become ready: error during reconciliation: "+errDescription)), "extensions indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			now := time.Now()
			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(ext.DeployAfterWorker(ctx)).To(Succeed())

			By("Patch object")
			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(afterWorkerExtension), afterWorkerExtension)).To(Succeed())
			patch := client.MergeFrom(afterWorkerExtension.DeepCopy())
			// remove operation annotation, add old timestamp annotation
			afterWorkerExtension.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
			}
			// set last operation
			afterWorkerExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(fakeSeedClient.Patch(ctx, afterWorkerExtension, patch)).ToNot(HaveOccurred(), "patching extension succeeds")

			By("Wait")
			Expect(ext.WaitAfterWorker(ctx)).NotTo(Succeed())
		})
	})

	Describe("#DestroyBeforeKubeAPIServer", func() {
		It("should not return error when not found", func() {
			Expect(ext.DestroyBeforeKubeAPIServer(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			for _, e := range allExtensions {
				Expect(fakeSeedClient.Create(ctx, e)).To(Succeed())
			}
			Expect(ext.DestroyBeforeKubeAPIServer(ctx)).To(Succeed())
			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(fakeSeedClient.List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(afterName))
		})

		It("should return error if deletion fails", func() {
			for _, e := range allExtensions {
				Expect(fakeSeedClient.Create(ctx, e)).To(Succeed())
			}
			fakeError := errors.New("fake-err")
			errClient := &errorClient{err: fakeError, Client: fakeSeedClient}
			ext = extension.New(
				log,
				errClient,
				&extension.Values{
					Namespace:  namespace.Name,
					Extensions: requiredExtensions,
				},
				time.Microsecond*100,
				time.Microsecond*400,
				time.Second,
			)
			Expect(ext.DestroyBeforeKubeAPIServer(ctx)).To(MatchError(error(&multierror.Error{Errors: []error{fakeError, fakeError, fakeError}})))
		})
	})

	Describe("#DestroyAfterKubeAPIServer", func() {
		It("should not return error when not found", func() {
			Expect(ext.DestroyAfterKubeAPIServer(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			for _, e := range allExtensions {
				Expect(fakeSeedClient.Create(ctx, e)).To(Succeed())
			}
			Expect(ext.DestroyAfterKubeAPIServer(ctx)).To(Succeed())
			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(fakeSeedClient.List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(defaultName, beforeName, afterWorkerName))
		})

		It("should return error if deletion fails", func() {
			for _, e := range allExtensions {
				Expect(fakeSeedClient.Create(ctx, e)).To(Succeed())
			}
			fakeError := errors.New("fake-err")
			errClient := &errorClient{err: fakeError, Client: fakeSeedClient}
			ext = extension.New(
				log,
				errClient,
				&extension.Values{
					Namespace:  namespace.Name,
					Extensions: requiredExtensions,
				},
				time.Microsecond*100,
				time.Microsecond*400,
				time.Second,
			)
			Expect(ext.DestroyAfterKubeAPIServer(ctx)).To(MatchError(error(&multierror.Error{Errors: []error{fakeError}})))
		})
	})

	Describe("#WaitCleanupBeforeKubeAPIServer", func() {
		It("should not return error if all resources are gone", func() {
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(ext.WaitCleanupBeforeKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if resources still exist", func() {
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(ext.WaitCleanupBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/before is still present")))
		})
	})

	Describe("#WaitCleanupAfterKubeAPIServer", func() {
		It("should not return error if all resources are gone", func() {
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(ext.WaitCleanupAfterKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if resources still exist", func() {
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(ext.WaitCleanupAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/after is still present")))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error if all resources are gone", func() {
			Expect(ext.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if resources still exist", func() {
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(ext.WaitCleanup(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/after is still present")))
		})
	})

	Describe("#RestoreBeforeKubeAPIServer", func() {
		var (
			state      = []byte(`{"dummy":"state"}`)
			shootState *gardencorev1beta1.ShootState
		)
		BeforeEach(func() {
			extensions := make([]gardencorev1beta1.ExtensionResourceState, 0, len(requiredExtensions))
			for _, ext := range requiredExtensions {
				extensions = append(extensions, gardencorev1beta1.ExtensionResourceState{
					Name:  ptr.To(ext.Name),
					Kind:  extensionsv1alpha1.ExtensionResource,
					State: &runtime.RawExtension{Raw: state},
				})
			}
			shootState = &gardencorev1beta1.ShootState{
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: extensions,
				},
			}
		})

		Describe("#RestoreBeforeKubeAPIServer", func() {
			It("should properly restore the extensions state if it exists", func() {
				Expect(ext.RestoreBeforeKubeAPIServer(ctx, shootState)).To(Succeed())

				extensionList := &extensionsv1alpha1.ExtensionList{}
				Expect(fakeSeedClient.List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
				Expect(extensionList.Items).To(consistOfObjects(beforeName))

				Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(beforeExtension), beforeExtension)).To(Succeed())
				Expect(beforeExtension.Status.State).To(Equal(&runtime.RawExtension{Raw: state}))
				Expect(beforeExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationRestore))
			})
		})

		Describe("#RestoreAfterKubeAPIServer", func() {
			It("should properly restore the extensions state if it exists", func() {
				Expect(ext.RestoreAfterKubeAPIServer(ctx, shootState)).To(Succeed())

				extensionList := &extensionsv1alpha1.ExtensionList{}
				Expect(fakeSeedClient.List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
				Expect(extensionList.Items).To(consistOfObjects(defaultName, afterName))

				Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(defaultExtension), defaultExtension)).To(Succeed())
				Expect(defaultExtension.Status.State).To(Equal(&runtime.RawExtension{Raw: state}))
				Expect(defaultExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationRestore))

				Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(afterExtension), afterExtension)).To(Succeed())
				Expect(afterExtension.Status.State).To(Equal(&runtime.RawExtension{Raw: state}))
				Expect(afterExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationRestore))
			})
		})

		Describe("#RestoreAfterWorker", func() {
			It("should properly restore the extensions state if it exists", func() {
				Expect(ext.RestoreAfterWorker(ctx, shootState)).To(Succeed())

				extensionList := &extensionsv1alpha1.ExtensionList{}
				Expect(fakeSeedClient.List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
				Expect(extensionList.Items).To(consistOfObjects(afterWorkerName))

				Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(afterWorkerExtension), afterWorkerExtension)).To(Succeed())
				Expect(afterWorkerExtension.Status.State).To(Equal(&runtime.RawExtension{Raw: state}))
				Expect(afterWorkerExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationRestore))
			})
		})
	})

	Describe("#MigrateBeforeKubeAPIServer", func() {
		It("should migrate the resources", func() {
			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(ext.MigrateBeforeKubeAPIServer(ctx)).To(Succeed())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(defaultExtension), defaultExtension)).To(Succeed())
			Expect(defaultExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(beforeExtension), beforeExtension)).To(Succeed())
			Expect(beforeExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(afterExtension), afterExtension)).To(Succeed())
			Expect(afterExtension.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())
		})

		It("should not return error if resource does not exist", func() {
			Expect(ext.MigrateBeforeKubeAPIServer(ctx)).To(Succeed())
		})
	})

	Describe("#MigrateAfterKubeAPIServer", func() {
		It("should migrate the resources", func() {
			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(ext.MigrateAfterKubeAPIServer(ctx)).To(Succeed())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(defaultExtension), defaultExtension)).To(Succeed())
			Expect(defaultExtension.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(beforeExtension), beforeExtension)).To(Succeed())
			Expect(beforeExtension.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(afterExtension), afterExtension)).To(Succeed())
			Expect(afterExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))
		})

		It("should not return error if resource does not exist", func() {
			Expect(ext.MigrateAfterKubeAPIServer(ctx)).To(Succeed())
		})
	})

	Describe("#WaitMigrateBeforeKubeAPIServer", func() {
		It("should not return error when resource is missing", func() {
			Expect(ext.WaitMigrateBeforeKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if resource is not yet migrated successfully", func() {
			defaultExtension.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			defaultExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())
			Expect(ext.WaitMigrateBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
		})

		It("should not return error if resource gets migrated successfully", func() {
			defaultExtension.Status.LastError = nil
			defaultExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(ext.WaitMigrateBeforeKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if one resources is not migrated successfully and others are", func() {
			defaultExtension.Status.LastError = nil
			defaultExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}
			beforeExtension.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			beforeExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(ext.WaitMigrateBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
		})
	})

	Describe("#WaitMigrateAfterKubeAPIServer", func() {
		It("should not return error when resource is missing", func() {
			Expect(ext.WaitMigrateAfterKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if resource is not yet migrated successfully", func() {
			afterExtension.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			afterExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(ext.WaitMigrateAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
		})

		It("should not return error if resource gets migrated successfully", func() {
			afterExtension.Status.LastError = nil
			afterExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(ext.WaitMigrateAfterKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if one resources is not migrated successfully and others are", func() {
			afterExtension1 := afterExtension.DeepCopy()
			afterExtension1.ObjectMeta.Name = "after1"
			afterExtension1.Spec.Type = "after1"
			afterExtension.Status.LastError = nil
			afterExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}
			afterExtension1.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			afterExtension1.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}
			afterKubeAPIServer := gardencorev1beta1.AfterKubeAPIServer
			requiredExtensions["after1"] = extension.Extension{
				Extension: *afterExtension1,
				Timeout:   time.Second,
				Lifecycle: &gardencorev1beta1.ControllerResourceLifecycle{
					Reconcile: &afterKubeAPIServer,
					Delete:    &afterKubeAPIServer,
					Migrate:   &afterKubeAPIServer,
				},
			}
			ext = extension.New(
				log,
				fakeSeedClient,
				&extension.Values{
					Namespace:  namespace.Name,
					Extensions: requiredExtensions,
				},
				time.Microsecond*100,
				time.Microsecond*400,
				time.Second,
			)

			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, afterExtension1)).To(Succeed())
			Expect(ext.WaitMigrateAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
		})
	})

	Describe("#DeleteResources", func() {
		It("should delete extensions resources", func() {
			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())

			Expect(ext.DeleteResources(ctx)).To(Succeed())

			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(fakeSeedClient.List(ctx, extensionList)).To(Succeed())
			Expect(extensionList.Items).To(BeEmpty())
		})
	})

	Describe("#WaitCleanupResources", func() {
		It("should not return error if all resources are gone", func() {
			Expect(ext.WaitCleanupStaleResources(ctx)).To(Succeed())
		})

		It("should return error if resources still exist", func() {
			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())

			Expect(ext.WaitCleanupResources(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/def is still present")))
		})
	})

	Describe("#DeleteStaleResources", func() {
		It("should delete stale extensions resources", func() {
			staleExtension := defaultExtension.DeepCopy()
			staleExtension.Name = "new-name"
			staleExtension.Spec.Type = "new-type"
			Expect(fakeSeedClient.Create(ctx, staleExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, beforeExtension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, afterExtension)).To(Succeed())

			Expect(ext.DeleteStaleResources(ctx)).To(Succeed())

			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(fakeSeedClient.List(ctx, extensionList)).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(defaultName, beforeName, afterName))
		})
	})

	Describe("#WaitCleanupStaleResources", func() {
		It("should not return error if all resources are gone", func() {
			Expect(ext.WaitCleanupStaleResources(ctx)).To(Succeed())
		})

		It("should not return error if wanted resources exist", func() {
			Expect(fakeSeedClient.Create(ctx, defaultExtension)).To(Succeed())
			Expect(ext.WaitCleanupStaleResources(ctx)).To(Succeed())
		})

		It("should return error if stale resources still exist", func() {
			staleExtension := defaultExtension
			staleExtension.Name = "new-name"
			staleExtension.Spec.Type = "new-type"
			Expect(fakeSeedClient.Create(ctx, staleExtension)).To(Succeed())

			Expect(ext.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/new-name is still present")))
		})
	})
})
