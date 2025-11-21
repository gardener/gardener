// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/provider-local/local"
	apiv1alpha1 "github.com/gardener/gardener/pkg/provider-local/machine-provider/api/v1alpha1"
)

func (d *localDriver) DeleteMachine(ctx context.Context, req *driver.DeleteMachineRequest) (*driver.DeleteMachineResponse, error) {
	if req.MachineClass.Provider != apiv1alpha1.Provider {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("requested for Provider '%s', we only support '%s'", req.MachineClass.Provider, apiv1alpha1.Provider))
	}

	providerClient, err := local.GetProviderClient(ctx, log, d.runtimeClient, *req.MachineClass.CredentialsSecretRef)
	if err != nil {
		return nil, fmt.Errorf("could not create client for infrastructure resources: %w", err)
	}

	klog.V(3).Infof("Machine deletion request has been received for %q", req.Machine.Name)
	defer klog.V(3).Infof("Machine deletion request has been processed for %q", req.Machine.Name)

	providerSpec, err := validateProviderSpecAndSecret(req.MachineClass, req.Secret)
	if err != nil {
		return nil, err
	}

	pod := podForMachine(req.Machine, req.MachineClass, providerSpec)
	if err := providerClient.Delete(ctx, pod); err != nil {
		if !apierrors.IsNotFound(err) {
			// Unknown leads to short retry in machine controller
			return nil, status.Error(codes.Unknown, fmt.Sprintf("error deleting pod: %s", err.Error()))
		}
		return nil, status.Error(codes.NotFound, err.Error())
	}

	// Actively wait until pod is deleted since the extension contract in machine-controller-manager expects drivers to
	// do so. If we would not wait until the pod is gone it might happen that the kubelet could re-register the Node
	// object even after it was already deleted by machine-controller-manager.
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := wait.PollUntilContextCancel(timeoutCtx, 5*time.Second, false, func(ctx context.Context) (bool, error) {
		if err := providerClient.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			// Unknown leads to short retry in machine controller
			return false, status.Error(codes.Unknown, err.Error())
		}
		return false, nil
	}); err != nil {
		// will be retried with short retry by machine controller
		return nil, status.Error(codes.DeadlineExceeded, err.Error())
	}

	return &driver.DeleteMachineResponse{}, nil
}
