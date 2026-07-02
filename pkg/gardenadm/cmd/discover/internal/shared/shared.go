// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	gonumgraph "gonum.org/v1/gonum/graph"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/graph"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// RunForShoot computes the required Gardener configuration resources for the given Shoot and exports them to the
// configured config directory. It is the shared backbone for both `gardenadm discover new` and
// `gardenadm discover existing`. The caller is responsible for loading the Shoot (from a manifest or the garden
// cluster) and for indicating whether backup resources (BackupBucket/BackupEntry) should be included.
func RunForShoot(
	ctx context.Context,
	opts *CommonOptions,
	c client.Client,
	fs afero.Afero,
	shoot *gardencorev1beta1.Shoot,
	fetchBackupResources bool,
) error {
	binding, err := secretBindingForShoot(ctx, c, shoot)
	if err != nil {
		return fmt.Errorf("failed reading binding for shoot: %w", err)
	}

	var (
		backupEntry  *gardencorev1beta1.BackupEntry
		backupBucket *gardencorev1beta1.BackupBucket
	)
	if fetchBackupResources {
		backupEntry, backupBucket, err = backupResourcesForShoot(ctx, c, shoot)
		if err != nil {
			return fmt.Errorf("failed reading backup resources for shoot: %w", err)
		}
		if backupBucket != nil && backupEntry == nil {
			fmt.Fprintf(opts.Out, "WARNING: found BackupBucket without a corresponding BackupEntry for Shoot %s - backup restoration may not be possible\n", client.ObjectKeyFromObject(shoot))
		}
	}

	fmt.Fprintf(opts.Out, "Computing required resources for Shoot...\n")

	g := graph.New(opts.Log, c, true)
	g.HandleShootCreateOrUpdate(ctx, shoot)
	if binding != nil {
		switch b := binding.(type) {
		case *gardencorev1beta1.SecretBinding:
			g.HandleSecretBindingCreateOrUpdate(b)
		case *securityv1alpha1.CredentialsBinding:
			g.HandleCredentialsBindingCreateOrUpdate(b)
		}
	}
	if backupEntry != nil {
		g.HandleBackupEntryCreateOrUpdate(backupEntry)
	}
	if backupBucket != nil {
		g.HandleBackupBucketCreateOrUpdate(backupBucket)
	}

	var taskFns []flow.TaskFn

	g.Visit(g.Nodes(), func(n gonumgraph.Node) {
		if vertex, ok := n.(*graph.Vertex); ok {
			taskFns = append(taskFns, func(ctx context.Context) error {
				kindObject, ok := graph.VertexTypes[vertex.Type]
				if !ok {
					return fmt.Errorf("unknown vertex type %q for vertex %s/%s", vertex.Type, vertex.Namespace, vertex.Name)
				}

				obj := kindObject.NewObjectFunc()
				obj.SetName(vertex.Name)
				obj.SetNamespace(vertex.Namespace)

				return getAndExportObject(ctx, c, fs, opts, kindObject.Kind, obj)
			})
		}
	})

	project, err := gardenerutils.ProjectForNamespaceFromReader(ctx, c, shoot.Namespace)
	if err != nil {
		return fmt.Errorf("failed reading project: %w", err)
	}
	taskFns = append(taskFns, func(ctx context.Context) error {
		return getAndExportObject(ctx, c, fs, opts, "Project", project)
	})

	extensions, err := requiredExtensions(ctx, c, shoot, opts.ManagedInfrastructure)
	if err != nil {
		return fmt.Errorf("failed computing required extensions: %w", err)
	}

	for _, extension := range extensions {
		taskFns = append(taskFns,
			func(ctx context.Context) error {
				return getAndExportObject(ctx, c, fs, opts, "ControllerRegistration", extension.ControllerRegistration)
			},
			func(ctx context.Context) error {
				return getAndExportObject(ctx, c, fs, opts, "ControllerDeployment", extension.ControllerDeployment)
			},
		)
	}

	fmt.Fprintf(opts.Out, "Fetching required resources for from garden cluster...\n\n")

	return flow.Parallel(taskFns...)(ctx)
}

func secretBindingForShoot(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot) (client.Object, error) {
	switch {
	case shoot.Spec.SecretBindingName != nil:
		secretBinding := &gardencorev1beta1.SecretBinding{ObjectMeta: metav1.ObjectMeta{Name: *shoot.Spec.SecretBindingName, Namespace: shoot.Namespace}}
		return secretBinding, c.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding)

	case shoot.Spec.CredentialsBindingName != nil:
		credentialsBinding := &securityv1alpha1.CredentialsBinding{ObjectMeta: metav1.ObjectMeta{Name: *shoot.Spec.CredentialsBindingName, Namespace: shoot.Namespace}}
		return credentialsBinding, c.Get(ctx, client.ObjectKeyFromObject(credentialsBinding), credentialsBinding)

	default:
		return nil, nil
	}
}

func backupResourcesForShoot(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.BackupEntry, *gardencorev1beta1.BackupBucket, error) {
	var (
		backupEntry  *gardencorev1beta1.BackupEntry
		backupBucket *gardencorev1beta1.BackupBucket
	)

	backupEntryList := &gardencorev1beta1.BackupEntryList{}
	if err := c.List(ctx, backupEntryList, client.InNamespace(shoot.Namespace),
		client.MatchingFields{gardencore.BackupEntryShootRefName: shoot.Name, gardencore.BackupEntryShootRefNamespace: shoot.Namespace}); err != nil {
		return nil, nil, fmt.Errorf("failed listing BackupEntries for Shoot %s: %w", client.ObjectKeyFromObject(shoot), err)
	}

	if len(backupEntryList.Items) > 1 {
		return nil, nil, fmt.Errorf("found more than one BackupEntry for Shoot %s", client.ObjectKeyFromObject(shoot))
	}
	if len(backupEntryList.Items) == 1 {
		backupEntry = &backupEntryList.Items[0]
	}

	backupBucketList := &gardencorev1beta1.BackupBucketList{}
	if err := c.List(ctx, backupBucketList,
		client.MatchingFields{gardencore.BackupBucketShootRefName: shoot.Name, gardencore.BackupBucketShootRefNamespace: shoot.Namespace}); err != nil {
		return nil, nil, fmt.Errorf("failed listing BackupBuckets for Shoot %s: %w", client.ObjectKeyFromObject(shoot), err)
	}

	if len(backupBucketList.Items) > 1 {
		return nil, nil, fmt.Errorf("found more than one BackupBucket for Shoot %s", client.ObjectKeyFromObject(shoot))
	}
	if len(backupBucketList.Items) == 1 {
		backupBucket = &backupBucketList.Items[0]
	}

	return backupEntry, backupBucket, nil
}

func requiredExtensions(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot, managedInfrastructure bool) ([]botanist.Extension, error) {
	resources := gardenadm.Resources{Shoot: shoot}

	controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
	if err := c.List(ctx, controllerRegistrationList); err != nil {
		return nil, fmt.Errorf("failed listing controllerRegistrations: %w", err)
	}
	controllerDeploymentList := &gardencorev1.ControllerDeploymentList{}
	if err := c.List(ctx, controllerDeploymentList); err != nil {
		return nil, fmt.Errorf("failed listing controllerDeployments: %w", err)
	}

	if err := meta.EachListItem(controllerRegistrationList, func(obj runtime.Object) error {
		resources.ControllerRegistrations = append(resources.ControllerRegistrations, obj.(*gardencorev1beta1.ControllerRegistration))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed adding ControllerRegistrations: %w", err)
	}

	if err := meta.EachListItem(controllerDeploymentList, func(obj runtime.Object) error {
		resources.ControllerDeployments = append(resources.ControllerDeployments, obj.(*gardencorev1.ControllerDeployment))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed adding ControllerDeployments: %w", err)
	}

	return botanist.ComputeExtensions(resources, true, managedInfrastructure)
}

func getAndExportObject(ctx context.Context, c client.Client, fs afero.Afero, opts *CommonOptions, kind string, obj client.Object) error {
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed getting %s %q: %w", kind, client.ObjectKeyFromObject(obj), err)
		}
		opts.Log.V(1).Info("Object not found in garden cluster", "kind", kind, "obj", client.ObjectKeyFromObject(obj))
		return nil
	}
	return exportObject(fs, opts, kind, obj)
}

func exportObject(fs afero.Afero, opts *CommonOptions, kind string, obj client.Object) error {
	resetObject(obj)

	objYAML, err := kubernetesutils.Serialize(obj, kubernetes.GardenScheme)
	if err != nil {
		return fmt.Errorf("failed serializing %T %q: %w", obj, client.ObjectKeyFromObject(obj), err)
	}

	path := filepath.Join(opts.ConfigDir, fmt.Sprintf("%s-%s.yaml", strings.ToLower(kind), obj.GetName()))
	if err := fs.WriteFile(path, []byte(objYAML), 0600); err != nil {
		return fmt.Errorf("failed writing file to %s: %w", path, err)
	}

	fmt.Fprintf(opts.Out, "Exported %s/%s to %s\n", kind, obj.GetName(), path)
	return nil
}

func resetObject(obj client.Object) {
	obj.SetCreationTimestamp(metav1.Time{})
	obj.SetFinalizers(nil)
	obj.SetGeneration(0)
	obj.SetOwnerReferences(nil)
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	obj.SetSelfLink("")
	obj.SetUID("")
}
