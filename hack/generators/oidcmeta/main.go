// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/util/keyutil"

	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "oidcmeta",
		Short: "A tool that can generate OpenID Configuration and JWKS from keys input.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	jwksCmd := &cobra.Command{
		Use:   "jwks",
		Short: "Generates JWKS.",
		RunE: func(_ *cobra.Command, _ []string) error {
			bytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed reading stdin: %w", err)
			}
			keys, err := keyutil.ParsePublicKeysPEM(bytes)
			if err != nil {
				return fmt.Errorf("could not parse public keys from file content: %w", err)
			}
			jwks, err := workloadidentity.JWKS(keys...)
			if err != nil {
				return fmt.Errorf("could not convert public keys to JWKS: %w", err)
			}
			fmt.Println(string(jwks))
			return nil
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Generates OpenID Config.",
		RunE: func(_ *cobra.Command, args []string) error {
			bytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed reading stdin: %w", err)
			}
			keys, err := keyutil.ParsePublicKeysPEM(bytes)
			if err != nil {
				return fmt.Errorf("could not parse public keys from file content: %w", err)
			}
			config, err := workloadidentity.OpenIDConfig(args[0], keys...)
			if err != nil {
				return fmt.Errorf("could not convert public keys to OpenID Configuration: %w", err)
			}
			fmt.Println(string(config))
			return nil
		},
	}
	configCmd.Args = cobra.MinimumNArgs(1)

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(jwksCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
