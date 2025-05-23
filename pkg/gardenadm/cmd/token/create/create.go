// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "create [token]",
		Short: "Create a bootstrap token on the cluster for joining a node",
		Long: `The [token] is the bootstrap token to be created on the cluster.
This token is used for securely authenticating nodes or clients to the cluster.
It must follow the format "[a-z0-9]{6}.[a-z0-9]{16}" to ensure compatibility with Kubernetes bootstrap token requirements.
If no [token] is provided, gardenadm will automatically generate a secure random token for you.`,

		Example: `# Create a bootstrap token with a specific ID and secret
gardenadm token create foo123.bar4567890baz123

# Create a bootstrap token with a specific ID and secret and directly print the gardenadm join command
gardenadm token create foo123.bar4567890baz123 --print-join-command

# Generate a random bootstrap token for joining a node
gardenadm token create`,

		Args: cobra.MaximumNArgs(1),

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

func run(ctx context.Context, opts *Options) error {
	clientSet, err := tokenutils.CreateClientSet(ctx, opts.Log)
	if err != nil {
		return fmt.Errorf("failed creating client set: %w", err)
	}

	if err := clientSet.Client().Get(ctx, client.ObjectKey{Name: bootstraptokenutil.BootstrapTokenSecretName(opts.Token.ID), Namespace: metav1.NamespaceSystem}, &corev1.Secret{}); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed checking if bootstrap token with ID %q already exists: %w", opts.Token.ID, err)
	} else if err == nil {
		return fmt.Errorf("bootstrap token with ID %q already exists", opts.Token.ID)
	}

	secret, err := bootstraptoken.ComputeBootstrapTokenWithSecret(ctx, clientSet.Client(), opts.Token.ID, opts.Token.Secret, opts.Description, opts.Validity)
	if err != nil {
		return fmt.Errorf("failed creating a bootstrap token secret: %w", err)
	}

	if !opts.PrintJoinCommand {
		fmt.Fprintln(opts.Out, bootstraptoken.FromSecretData(secret.Data))
		return nil
	}

	output, err := printJoinCommand(ctx, opts, clientSet, secret)
	if err != nil {
		return fmt.Errorf("failed computing join command: %w", err)
	}
	fmt.Fprint(opts.Out, output)

	return nil
}

func printJoinCommand(ctx context.Context, opts *Options, clientSet kubernetes.Interface, bootstrapTokenSecret *corev1.Secret) (string, error) {
	secretList := &corev1.SecretList{}
	if err := clientSet.Client().List(ctx, secretList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{
		v1beta1constants.GardenRole:      v1beta1constants.GardenRoleOperatingSystemConfig,
		v1beta1constants.LabelWorkerPool: opts.WorkerPoolName,
	}); err != nil {
		return "", fmt.Errorf("failed listing gardener-node-agent secrets: %w", err)
	}

	if len(secretList.Items) == 0 {
		return "", fmt.Errorf("no gardener-node-agent secrets found for worker pool %q", opts.WorkerPoolName)
	}

	gardenerNodeAgentSecret := secretList.Items[0]
	if len(secretList.Items) > 1 {
		opts.Log.V(1).Info("Multiple gardener-node-agent secrets found, using the first one", "secretName", gardenerNodeAgentSecret.Name)
	}

	return fmt.Sprintf(`gardenadm join --bootstrap-token %s --ca-certificate "%s" --gardener-node-agent-secret-name %s %s
`,
		bootstraptoken.FromSecretData(bootstrapTokenSecret.Data),
		utils.EncodeBase64(clientSet.RESTConfig().CAData),
		gardenerNodeAgentSecret.Name,
		clientSet.RESTConfig().Host,
	), nil
}
