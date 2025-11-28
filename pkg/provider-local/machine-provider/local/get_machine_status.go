// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"fmt"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/provider-local/local"
	apiv1alpha1 "github.com/gardener/gardener/pkg/provider-local/machine-provider/api/v1alpha1"
)

func (d *localDriver) GetMachineStatus(ctx context.Context, req *driver.GetMachineStatusRequest) (*driver.GetMachineStatusResponse, error) {
	if req.MachineClass.Provider != apiv1alpha1.Provider {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("requested for Provider '%s', we only support '%s'", req.MachineClass.Provider, apiv1alpha1.Provider))
	}

	providerClient, err := local.GetProviderClient(ctx, log, d.runtimeClient, *req.MachineClass.CredentialsSecretRef)
	if err != nil {
		return nil, fmt.Errorf("could not create client for infrastructure resources: %w", err)
	}

	klog.V(3).Infof("Machine status request has been received for %q", req.Machine.Name)
	defer klog.V(3).Infof("Machine status request has been processed for %q", req.Machine.Name)

	providerSpec, err := validateProviderSpecAndSecret(req.MachineClass, req.Secret)
	if err != nil {
		return nil, err
	}

	pod := podForMachine(req.Machine, req.MachineClass, providerSpec)
	if err := providerClient.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &driver.GetMachineStatusResponse{
		ProviderID: pod.Name,
		NodeName:   pod.Name,
		Addresses:  addressesFromStatus(pod.Status),
	}, nil
}
