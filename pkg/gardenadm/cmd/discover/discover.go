// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	gonumgraph "gonum.org/v1/gonum/graph"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/graph"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Conveniently download Gardener configuration resources from an existing garden cluster",
		Long:  "Conveniently download Gardener configuration resources from an existing garden cluster (CloudProfile, ControllerRegistrations, ControllerDeployments, etc.)",

		Example: `# Download the configuration
gardenadm discover <path-to-shoot-manifest>`,

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.ParseArgs(args); err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			if err := opts.Complete(); err != nil {
				return err
			}

			return run(cmd.Context(), opts)
		},
	}

	opts.addFlags(cmd.Flags())

	return cmd
}

var (
	// NewClientSetFromFile is an alias for botanist.NewClientSetFromFile.
	// Exposed for unit testing.
	NewClientSetFromFile = botanist.NewClientSetFromFile
	// NewAferoFs is an alias for returning an afero.NewOsFs.
	// Exposed for unit testing.
	NewAferoFs = func() afero.Afero { return afero.Afero{Fs: afero.NewOsFs()} }
)

func run(ctx context.Context, opts *Options) error {
	fs := NewAferoFs()

	shoot, err := readShoot(fs, opts.ShootManifest)
	if err != nil {
		return fmt.Errorf("failed reading shoot manifest from %q: %w", opts.ShootManifest, err)
	}

	clientSet, err := NewClientSetFromFile(opts.Kubeconfig, kubernetes.GardenScheme)
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}

	binding, err := secretBindingForShoot(ctx, clientSet.Client(), shoot)
	if err != nil {
		return fmt.Errorf("failed reading binding for shoot: %w", err)
	}

	fmt.Fprintf(opts.Out, "Computing required resources for Shoot...\n")

	g := graph.New(opts.Log, clientSet.Client())
	g.HandleShootCreateOrUpdate(ctx, shoot)
	if binding != nil {
		switch b := binding.(type) {
		case *gardencorev1beta1.SecretBinding:
			g.HandleSecretBindingCreateOrUpdate(b)
		case *securityv1alpha1.CredentialsBinding:
			g.HandleCredentialsBindingCreateOrUpdate(b)
		}
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

				return getAndExportObject(ctx, clientSet.Client(), fs, opts, kindObject.Kind, obj)
			})
		}
	})

	project, err := gardenerutils.ProjectForNamespaceFromReader(ctx, clientSet.Client(), shoot.Namespace)
	if err != nil {
		return fmt.Errorf("failed reading project: %w", err)
	}
	taskFns = append(taskFns, func(ctx context.Context) error {
		return getAndExportObject(ctx, clientSet.Client(), fs, opts, "Project", project)
	})

	extensions, err := requiredExtensions(ctx, clientSet.Client(), shoot, opts.RunsControlPlane)
	if err != nil {
		return fmt.Errorf("failed computing required extensions: %w", err)
	}

	for _, extension := range extensions {
		taskFns = append(taskFns,
			func(ctx context.Context) error {
				return getAndExportObject(ctx, clientSet.Client(), fs, opts, "ControllerRegistration", extension.ControllerRegistration)
			},
			func(ctx context.Context) error {
				return getAndExportObject(ctx, clientSet.Client(), fs, opts, "ControllerDeployment", extension.ControllerDeployment)
			},
		)
	}

	fmt.Fprintf(opts.Out, "Fetching required resources for from garden cluster...\n\n")

	return flow.Parallel(taskFns...)(ctx)
}

var (
	versions = schema.GroupVersions([]schema.GroupVersion{gardencorev1.SchemeGroupVersion, gardencorev1beta1.SchemeGroupVersion})
	decoder  = kubernetes.GardenCodec.CodecForVersions(kubernetes.GardenSerializer, kubernetes.GardenSerializer, versions, versions)
)

func readShoot(fs afero.Afero, manifestPath string) (*gardencorev1beta1.Shoot, error) {
	shootManifest, err := fs.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	shoot := &gardencorev1beta1.Shoot{}
	if _, _, err := decoder.Decode(shootManifest, nil, shoot); err != nil {
		return nil, err
	}

	return shoot, nil
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

func requiredExtensions(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot, runsControlPlane bool) ([]botanist.Extension, error) {
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

	return botanist.ComputeExtensions(resources, true, runsControlPlane)
}

func getAndExportObject(ctx context.Context, c client.Client, fs afero.Afero, opts *Options, kind string, obj client.Object) error {
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed getting %s %q: %w", kind, client.ObjectKeyFromObject(obj), err)
		}
		opts.Log.V(1).Info("Object not found in garden cluster", "kind", kind, "obj", client.ObjectKeyFromObject(obj))
		return nil
	}

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
