// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/printers"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Display a list of all bootstrap tokens available on the cluster.",
		Long: `The "list" command retrieves and displays all bootstrap tokens currently available on the cluster. 
Bootstrap tokens are used for authenticating nodes during the join process.`,

		Example: `# To list all bootstrap tokens available on the cluster:
gardenadm token list

# To include additional sensitive details such as token secrets:
gardenadm token list --with-token-secret`,

		Args: cobra.ExactArgs(0),

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

// Now computes the current time.
// Exposed for unit testing.
var Now = metav1.Now

func run(ctx context.Context, opts *Options) error {
	clientSet, err := tokenutils.CreateClientSet(ctx, opts.Log)
	if err != nil {
		return fmt.Errorf("failed creating client set: %w", err)
	}

	secretList := &corev1.SecretList{}
	if err := clientSet.Client().List(ctx, secretList, client.MatchingFields{"type": string(bootstraptokenapi.SecretTypeBootstrapToken)}); err != nil {
		return fmt.Errorf("failed listing bootstrap token secrets: %w", err)
	}

	if len(secretList.Items) == 0 {
		fmt.Fprintln(opts.Out, "No resources found.")
		return nil
	}

	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "NAME", Type: "string", Format: "name", Description: "Name of the bootstrap token secret"},
			{Name: "TOKEN ID", Type: "string", Description: "Bootstrap token ID"},
			{Name: "EXPIRATION", Type: "string", Description: "Expiration of the bootstrap token"},
			{Name: "DESCRIPTION", Type: "string", Description: "Description of the bootstrap token"},
			{Name: "AGE", Type: "string", Description: "Age of the bootstrap token"},
		},
		Rows: make([]metav1.TableRow, 0, len(secretList.Items)),
	}

	if opts.WithTokenSecret {
		table.ColumnDefinitions = append(table.ColumnDefinitions[:2], append([]metav1.TableColumnDefinition{
			{Name: "TOKEN SECRET", Type: "string", Description: "Bootstrap token secret"},
			{Name: "TOKEN", Type: "string", Description: "Bootstrap token"},
		}, table.ColumnDefinitions[2:]...)...)
	}

	for _, secret := range secretList.Items {
		expirationTime, err := time.Parse(time.RFC3339, string(secret.Data[bootstraptokenapi.BootstrapTokenExpirationKey]))
		if err != nil {
			return fmt.Errorf("failed parsing the expiration time %q for secret %s: %w", secret.Data[bootstraptokenapi.BootstrapTokenExpirationKey], client.ObjectKeyFromObject(&secret), err)
		}

		row := metav1.TableRow{Cells: []any{
			secret.Name,
			string(secret.Data[bootstraptokenapi.BootstrapTokenIDKey]),
			fmt.Sprintf("%s (%s)", duration.HumanDuration(expirationTime.UTC().Sub(Now().UTC())), expirationTime.UTC().Format("2006-01-02T15:04:05Z")),
			string(secret.Data[bootstraptokenapi.BootstrapTokenDescriptionKey]),
			metatable.ConvertToHumanReadableDateType(secret.CreationTimestamp),
		}}

		if opts.WithTokenSecret {
			row.Cells = append(row.Cells[:2], append([]any{
				string(secret.Data[bootstraptokenapi.BootstrapTokenSecretKey]),
				bootstraptoken.FromSecretData(secret.Data),
			}, row.Cells[2:]...)...)
		}

		table.Rows = append(table.Rows, row)
	}

	return printers.NewTablePrinter(printers.PrintOptions{}).PrintObj(table, opts.Out)
}
