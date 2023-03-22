// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/extension"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

type errorClient struct {
	client.Client
	err error
}

func (e *errorClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return e.err
}

var (
	objectIdentifier = Identifier(func(obj interface{}) string {
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
		defaultName = "def"
		afterName   = "after"
		beforeName  = "before"
	)
	var (
		fakeSeedClient   client.Client
		fakeGardenClient client.Client
		b                *botanist.Botanist
		namespace        *corev1.Namespace
		ctx              = context.TODO()
		shootState       = &gardencorev1beta1.ShootState{}
		log              logr.Logger

		defaultExtension *extensionsv1alpha1.Extension
		beforeExtension  *extensionsv1alpha1.Extension
		afterExtension   *extensionsv1alpha1.Extension
		allExtensions    []*extensionsv1alpha1.Extension

		requiredExtensions map[string]extension.Extension
	)

	BeforeEach(func() {
		fakeGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSeedClientSet := kubernetesfake.NewClientSetBuilder().WithClient(fakeSeedClient).Build()
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"}}

		logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
		log = logf.Log.WithName("extensions")

		b = &botanist.Botanist{Operation: &operation.Operation{
			GardenClient:  fakeGardenClient,
			SeedClientSet: fakeSeedClientSet,
			Logger:        log,
			Shoot: &shootpkg.Shoot{
				SeedNamespace: namespace.Name,
				Components:    &shootpkg.Components{Extensions: &shootpkg.Extensions{}},
			},
		}}
		b.SetShootState(shootState)
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{})

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

		allExtensions = []*extensionsv1alpha1.Extension{defaultExtension, beforeExtension, afterExtension}

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
		}

		b.Shoot.Components.Extensions.Extension = extension.New(
			log,
			b.SeedClientSet.Client(),
			&extension.Values{
				Namespace:  b.Shoot.SeedNamespace,
				Extensions: requiredExtensions,
			},
			time.Microsecond*100,
			time.Microsecond*400,
			time.Second,
		)
	})

	Describe("#DeployBeforeKubeAPIServer", func() {
		It("should successfully deploy extension resources", func() {
			Expect(b.Shoot.Components.Extensions.Extension.DeployBeforeKubeAPIServer(ctx)).To(Succeed())
			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(b.SeedClientSet.Client().List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(beforeName))
		})
	})

	Describe("#DeployAfterKubeAPIServer", func() {
		It("should successfully deploy extension resources", func() {
			Expect(b.Shoot.Components.Extensions.Extension.DeployAfterKubeAPIServer(ctx)).To(Succeed())
			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(b.SeedClientSet.Client().List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(defaultName, afterName))
		})
	})

	Describe("#WaitBeforeKubeAPIServer", func() {
		It("should return error when no resources are found", func() {
			Expect(b.Shoot.Components.Extensions.Extension.WaitBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("not found")))
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
			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, beforeExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("Error while waiting for Extension test-namespace/before to become ready: error during reconciliation: "+errDescription)), "extensions indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			now := time.Now()
			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(b.Shoot.Components.Extensions.Extension.DeployBeforeKubeAPIServer(ctx)).To(Succeed())

			By("Patch object")
			Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(beforeExtension), beforeExtension)).To(Succeed())
			patch := client.MergeFrom(beforeExtension.DeepCopy())
			// remove operation annotation, add old timestamp annotation
			beforeExtension.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().String(),
			}
			// set last operation
			beforeExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(b.SeedClientSet.Client().Patch(ctx, beforeExtension, patch)).ToNot(HaveOccurred(), "patching extension succeeds")

			By("Wait")
			Expect(b.Shoot.Components.Extensions.Extension.WaitBeforeKubeAPIServer(ctx)).NotTo(Succeed())
		})
	})

	Describe("#WaitAfterKubeAPIServer", func() {
		It("should return error when no resources are found", func() {
			Expect(b.Shoot.Components.Extensions.Extension.WaitAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("not found")))
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
			Expect(b.SeedClientSet.Client().Create(ctx, defaultExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, beforeExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("Error while waiting for Extension test-namespace/def to become ready: error during reconciliation: "+errDescription)), "extensions indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			now := time.Now()
			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(b.Shoot.Components.Extensions.Extension.DeployAfterKubeAPIServer(ctx)).To(Succeed())

			By("Patch object")
			Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(defaultExtension), defaultExtension)).To(Succeed())
			patch := client.MergeFrom(defaultExtension.DeepCopy())
			// remove operation annotation, add old timestamp annotation
			defaultExtension.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().String(),
			}
			// set last operation
			defaultExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(b.SeedClientSet.Client().Patch(ctx, defaultExtension, patch)).ToNot(HaveOccurred(), "patching extension succeeds")

			By("Wait")
			Expect(b.Shoot.Components.Extensions.Extension.WaitAfterKubeAPIServer(ctx)).NotTo(Succeed())
		})
	})

	Describe("#DestroyBeforeKubeAPIServer", func() {
		It("should not return error when not found", func() {
			Expect(b.Shoot.Components.Extensions.Extension.DestroyBeforeKubeAPIServer(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			for _, e := range allExtensions {
				Expect(b.SeedClientSet.Client().Create(ctx, e)).To(Succeed())
			}
			Expect(b.Shoot.Components.Extensions.Extension.DestroyBeforeKubeAPIServer(ctx)).To(Succeed())
			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(b.SeedClientSet.Client().List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(afterName))
		})

		It("should return error if deletion fails", func() {
			for _, e := range allExtensions {
				Expect(b.SeedClientSet.Client().Create(ctx, e)).To(Succeed())
			}
			fakeError := errors.New("fake-err")
			errClient := &errorClient{err: fakeError, Client: b.SeedClientSet.Client()}
			errClientSet := kubernetesfake.NewClientSetBuilder().WithClient(errClient).Build()
			b.SeedClientSet = errClientSet
			b.Shoot.Components.Extensions.Extension = extension.New(
				log,
				b.SeedClientSet.Client(),
				&extension.Values{
					Namespace:  b.Shoot.SeedNamespace,
					Extensions: requiredExtensions,
				},
				time.Microsecond*100,
				time.Microsecond*400,
				time.Second,
			)
			Expect(b.Shoot.Components.Extensions.Extension.DestroyBeforeKubeAPIServer(ctx)).To(MatchError(&multierror.Error{
				Errors: []error{fakeError, fakeError},
			}))
		})
	})

	Describe("#DestroyAfterKubeAPIServer", func() {
		It("should not return error when not found", func() {
			Expect(b.Shoot.Components.Extensions.Extension.DestroyAfterKubeAPIServer(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			for _, e := range allExtensions {
				Expect(b.SeedClientSet.Client().Create(ctx, e)).To(Succeed())
			}
			Expect(b.Shoot.Components.Extensions.Extension.DestroyAfterKubeAPIServer(ctx)).To(Succeed())
			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(b.SeedClientSet.Client().List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(defaultName, beforeName))
		})

		It("should return error if deletion fails", func() {
			for _, e := range allExtensions {
				Expect(b.SeedClientSet.Client().Create(ctx, e)).To(Succeed())
			}
			fakeError := errors.New("fake-err")
			errClient := &errorClient{err: fakeError, Client: b.SeedClientSet.Client()}
			errClientSet := kubernetesfake.NewClientSetBuilder().WithClient(errClient).Build()
			b.SeedClientSet = errClientSet
			b.Shoot.Components.Extensions.Extension = extension.New(
				log,
				b.SeedClientSet.Client(),
				&extension.Values{
					Namespace:  b.Shoot.SeedNamespace,
					Extensions: requiredExtensions,
				},
				time.Microsecond*100,
				time.Microsecond*400,
				time.Second,
			)
			Expect(b.Shoot.Components.Extensions.Extension.DestroyAfterKubeAPIServer(ctx)).To(MatchError(&multierror.Error{
				Errors: []error{fakeError},
			}))
		})
	})

	Describe("#WaitCleanupBeforeKubeAPIServer", func() {
		It("should not return error if all resources are gone", func() {
			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitCleanupBeforeKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if resources still exist", func() {
			Expect(b.SeedClientSet.Client().Create(ctx, beforeExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitCleanupBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/before is still present")))
		})
	})

	Describe("#WaitCleanupAfterKubeAPIServer", func() {
		It("should not return error if all resources are gone", func() {
			Expect(b.SeedClientSet.Client().Create(ctx, beforeExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitCleanupAfterKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if resources still exist", func() {
			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitCleanupAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/after is still present")))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error if all resources are gone", func() {
			Expect(b.Shoot.Components.Extensions.Extension.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if resources still exist", func() {
			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitCleanup(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/after is still present")))
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
					Name:  pointer.String(ext.Name),
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
				Expect(b.Shoot.Components.Extensions.Extension.RestoreBeforeKubeAPIServer(ctx, shootState)).To(Succeed())

				extensionList := &extensionsv1alpha1.ExtensionList{}
				Expect(b.SeedClientSet.Client().List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
				Expect(extensionList.Items).To(consistOfObjects(beforeName))

				Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(beforeExtension), beforeExtension)).To(Succeed())
				Expect(beforeExtension.Status.State).To(Equal(&runtime.RawExtension{Raw: state}))
				Expect(beforeExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationRestore))
			})
		})

		Describe("#RestoreAfterKubeAPIServer", func() {
			It("should properly restore the extensions state if it exists", func() {
				Expect(b.Shoot.Components.Extensions.Extension.RestoreAfterKubeAPIServer(ctx, shootState)).To(Succeed())

				extensionList := &extensionsv1alpha1.ExtensionList{}
				Expect(b.SeedClientSet.Client().List(ctx, extensionList, client.InNamespace(namespace.Name))).To(Succeed())
				Expect(extensionList.Items).To(consistOfObjects(defaultName, afterName))

				Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(defaultExtension), defaultExtension)).To(Succeed())
				Expect(defaultExtension.Status.State).To(Equal(&runtime.RawExtension{Raw: state}))
				Expect(defaultExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationRestore))

				Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(afterExtension), afterExtension)).To(Succeed())
				Expect(afterExtension.Status.State).To(Equal(&runtime.RawExtension{Raw: state}))
				Expect(afterExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationRestore))
			})
		})
	})

	Describe("#MigrateBeforeKubeAPIServer", func() {
		It("should migrate the resources", func() {
			Expect(b.SeedClientSet.Client().Create(ctx, defaultExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, beforeExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.MigrateBeforeKubeAPIServer(ctx)).To(Succeed())

			Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(defaultExtension), defaultExtension)).To(Succeed())
			Expect(defaultExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))

			Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(beforeExtension), beforeExtension)).To(Succeed())
			Expect(beforeExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))

			Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(afterExtension), afterExtension)).To(Succeed())
			Expect(afterExtension.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())
		})

		It("should not return error if resource does not exist", func() {
			Expect(b.Shoot.Components.Extensions.Extension.MigrateBeforeKubeAPIServer(ctx)).To(Succeed())
		})
	})

	Describe("#MigrateAfterKubeAPIServer", func() {
		It("should migrate the resources", func() {
			Expect(b.SeedClientSet.Client().Create(ctx, defaultExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, beforeExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.MigrateAfterKubeAPIServer(ctx)).To(Succeed())

			Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(defaultExtension), defaultExtension)).To(Succeed())
			Expect(defaultExtension.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())

			Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(beforeExtension), beforeExtension)).To(Succeed())
			Expect(beforeExtension.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())

			Expect(b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(afterExtension), afterExtension)).To(Succeed())
			Expect(afterExtension.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))
		})

		It("should not return error if resource does not exist", func() {
			Expect(b.Shoot.Components.Extensions.Extension.MigrateAfterKubeAPIServer(ctx)).To(Succeed())
		})
	})

	Describe("#WaitMigrateBeforeKubeAPIServer", func() {
		It("should not return error when resource is missing", func() {
			Expect(b.Shoot.Components.Extensions.Extension.WaitMigrateBeforeKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if resource is not yet migrated successfully", func() {
			defaultExtension.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			defaultExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(b.SeedClientSet.Client().Create(ctx, defaultExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitMigrateBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
		})

		It("should not return error if resource gets migrated successfully", func() {
			defaultExtension.Status.LastError = nil
			defaultExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(b.SeedClientSet.Client().Create(ctx, defaultExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitMigrateBeforeKubeAPIServer(ctx)).To(Succeed())
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

			Expect(b.SeedClientSet.Client().Create(ctx, defaultExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, beforeExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitMigrateBeforeKubeAPIServer(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
		})
	})

	Describe("#WaitMigrateAfterKubeAPIServer", func() {
		It("should not return error when resource is missing", func() {
			Expect(b.Shoot.Components.Extensions.Extension.WaitMigrateAfterKubeAPIServer(ctx)).To(Succeed())
		})

		It("should return error if resource is not yet migrated successfully", func() {
			afterExtension.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			afterExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitMigrateAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
		})

		It("should not return error if resource gets migrated successfully", func() {
			afterExtension.Status.LastError = nil
			afterExtension.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, beforeExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitMigrateAfterKubeAPIServer(ctx)).To(Succeed())
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
			b.Shoot.Components.Extensions.Extension = extension.New(
				log,
				b.SeedClientSet.Client(),
				&extension.Values{
					Namespace:  b.Shoot.SeedNamespace,
					Extensions: requiredExtensions,
				},
				time.Microsecond*100,
				time.Microsecond*400,
				time.Second,
			)

			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension1)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitMigrateAfterKubeAPIServer(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
		})
	})

	Describe("#DeleteStaleResources", func() {
		It("should delete stale extensions resources", func() {
			staleExtension := defaultExtension.DeepCopy()
			staleExtension.Name = "new-name"
			staleExtension.Spec.Type = "new-type"
			Expect(b.SeedClientSet.Client().Create(ctx, staleExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, defaultExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, beforeExtension)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, afterExtension)).To(Succeed())

			Expect(b.Shoot.Components.Extensions.Extension.DeleteStaleResources(ctx)).To(Succeed())

			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(b.SeedClientSet.Client().List(ctx, extensionList)).To(Succeed())
			Expect(extensionList.Items).To(consistOfObjects(defaultName, beforeName, afterName))
		})
	})
	Describe("#WaitCleanupStaleResources", func() {
		It("should not return error if all resources are gone", func() {
			Expect(b.Shoot.Components.Extensions.Extension.WaitCleanupStaleResources(ctx)).To(Succeed())
		})

		It("should not return error if wanted resources exist", func() {
			Expect(b.SeedClientSet.Client().Create(ctx, defaultExtension)).To(Succeed())
			Expect(b.Shoot.Components.Extensions.Extension.WaitCleanupStaleResources(ctx)).To(Succeed())
		})

		It("should return error if stale resources still exist", func() {
			staleExtension := defaultExtension
			staleExtension.Name = "new-name"
			staleExtension.Spec.Type = "new-type"
			Expect(b.SeedClientSet.Client().Create(ctx, staleExtension)).To(Succeed())

			Expect(b.Shoot.Components.Extensions.Extension.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/new-name is still present")))
		})
	})
})
