// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	sshutils "github.com/gardener/gardener/pkg/utils/ssh"
)

// ConnectToControlPlaneMachine opens an SSH connection via the Bastion to the first control plane machine of the
// self-hosted shoot. The connection is stored in the sshConnection field.
func (b *GardenadmBotanist) ConnectToControlPlaneMachine(ctx context.Context) error {
	machine, err := b.GetMachineByIndex(0)
	if err != nil {
		return err
	}

	machineAddr, err := PreferredAddressForMachine(machine)
	if err != nil {
		return err
	}
	sshAddr := net.JoinHostPort(machineAddr, "22")

	sshKeypairSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameSSHKeyPair)
	}

	conn, err := sshutils.Dial(ctx, sshAddr,
		sshutils.WithProxyConnection(b.Bastion.Connection),
		sshutils.WithUser("gardener"),
		sshutils.WithPrivateKeyBytes(sshKeypairSecret.Data[secretsutils.DataKeyRSAPrivateKey]),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to machine %q at %q via bastion: %w", machine.Name, sshAddr, err)
	}

	// We need root permissions for running gardenadm commands on the machine.
	// Also, add a line prefix to distinguish output from different machines.
	b.sshConnection = conn.RunAsUser("root").WithOutputPrefix(fmt.Sprintf("[%s] ", machine.Name))

	return nil
}

// SSHConnection returns the SSH connection to the control plane machine opened by ConnectToControlPlaneMachine.
func (b *GardenadmBotanist) SSHConnection() *sshutils.Connection {
	return b.sshConnection
}

var (
	// NB: We don't use filepath.Join here, because we explicitly need Linux path separators for the target machine,
	// even when running `gardenadm bootstrap` on Windows.

	// ImageVectorOverrideFile is the path where the image vector overwrite is copied to on the control plane machine.
	ImageVectorOverrideFile = GardenadmBaseDir + "/imagevector-overwrite.yaml"
	// ManifestsDir is the path where the manifests are copied to on the control plane machine.
	ManifestsDir = GardenadmBaseDir + "/manifests"

	manifestFilePermissions = "0600"
)

// CopyManifests copies all manifests needed for `gardenadm init` to the remote machine under GardenadmBaseDir.
func (b *GardenadmBotanist) CopyManifests(ctx context.Context, configDir fs.FS) error {
	if err := prepareRemoteDirs(ctx, b.sshConnection); err != nil {
		return err
	}

	// Copy all manifest files from --config-dir
	if err := gardenadm.VisitManifestFiles(configDir, func(path string, file fs.File) error {
		if err := b.sshConnection.CopyFile(ctx, filepath.Join(ManifestsDir, path), manifestFilePermissions, file); err != nil {
			return fmt.Errorf("failed copying file %s: %w", path, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("error copying manifests: %w", err)
	}

	if err := copyImageVectorOverride(ctx, b.sshConnection); err != nil {
		return err
	}

	return b.copyShootState(ctx)
}

func prepareRemoteDirs(ctx context.Context, conn *sshutils.Connection) error {
	// Empty manifests dir to start with a clean state on re-runs
	if _, _, err := conn.Run(ctx, "rm -rf "+ManifestsDir); err != nil {
		return fmt.Errorf("error removing manifests dir: %w", err)
	}
	if _, _, err := conn.Run(ctx, "mkdir -p "+ManifestsDir); err != nil {
		return fmt.Errorf("error ensuring manifests dir: %w", err)
	}

	return nil
}

func copyImageVectorOverride(ctx context.Context, conn *sshutils.Connection) (err error) {
	imageVectorOverride := os.Getenv(imagevector.OverrideEnv)
	if imageVectorOverride == "" {
		return nil
	}

	file, err := os.Open(imageVectorOverride) // #nosec: G304 -- ImageVectorOverwrite is a feature.
	if err != nil {
		return fmt.Errorf("error opening image vector overwrite file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	if err := conn.CopyFile(ctx, ImageVectorOverrideFile, manifestFilePermissions, file); err != nil {
		return fmt.Errorf("error copying image vector overwrite file: %w", err)
	}

	return nil
}

func (b *GardenadmBotanist) copyShootState(ctx context.Context) error {
	shootState := &gardencorev1beta1.ShootState{}
	if err := b.GardenClient.Get(ctx, client.ObjectKeyFromObject(b.Shoot.GetInfo()), shootState); err != nil {
		return fmt.Errorf("error getting ShootState: %w", err)
	}

	// Clear fields that must not be set on creation
	shootState.SetResourceVersion("")

	shootStateBytes, err := runtime.Encode(kubernetes.GardenCodec.EncoderForVersion(kubernetes.GardenSerializer, gardencorev1beta1.SchemeGroupVersion), shootState)
	if err != nil {
		return fmt.Errorf("error encoding ShootState: %w", err)
	}

	if err := b.sshConnection.Copy(ctx, filepath.Join(ManifestsDir, "shootstate.yaml"), manifestFilePermissions, shootStateBytes); err != nil {
		return fmt.Errorf("error copying ShootState: %w", err)
	}

	return nil
}
