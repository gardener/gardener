// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package genericmutator

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/cloudinit"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/coreos/go-systemd/unit"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// EnsurerContext wraps the actual context and cluster object.
type EnsurerContext interface {
	GetCluster(ctx context.Context) (*extensionscontroller.Cluster, error)
}

// Ensurer ensures that various standard Kubernets controlplane objects conform to the provider requirements.
// If they don't initially, they are mutated accordingly.
type Ensurer interface {
	// EnsureKubeAPIServerService ensures that the kube-apiserver service conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubeAPIServerService(ctx context.Context, etcx EnsurerContext, new, old *corev1.Service) error
	// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubeAPIServerDeployment(ctx context.Context, etcx EnsurerContext, new, old *appsv1.Deployment) error
	// EnsureKubeControllerManagerDeployment ensures that the kube-controller-manager deployment conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubeControllerManagerDeployment(ctx context.Context, etcx EnsurerContext, new, old *appsv1.Deployment) error
	// EnsureKubeSchedulerDeployment ensures that the kube-scheduler deployment conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubeSchedulerDeployment(ctx context.Context, etcx EnsurerContext, new, old *appsv1.Deployment) error
	// EnsureETCD ensures that the etcds conform to the respective provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureETCD(ctx context.Context, etcx EnsurerContext, new, old *druidv1alpha1.Etcd) error
	// EnsureKubeletServiceUnitOptions ensures that the kubelet.service unit options conform to the provider requirements.
	EnsureKubeletServiceUnitOptions(ctx context.Context, etcx EnsurerContext, new, old []*unit.UnitOption) ([]*unit.UnitOption, error)
	// EnsureKubeletConfiguration ensures that the kubelet configuration conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubeletConfiguration(ctx context.Context, etcx EnsurerContext, new, old *kubeletconfigv1beta1.KubeletConfiguration) error
	// EnsureKubernetesGeneralConfiguration ensures that the kubernetes general configuration conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubernetesGeneralConfiguration(ctx context.Context, etcx EnsurerContext, new, old *string) error
	// ShouldProvisionKubeletCloudProviderConfig returns true if the cloud provider config file should be added to the kubelet configuration.
	ShouldProvisionKubeletCloudProviderConfig(ctx context.Context, etcx EnsurerContext) bool
	// EnsureKubeletCloudProviderConfig ensures that the cloud provider config file content conforms to the provider requirements.
	EnsureKubeletCloudProviderConfig(context.Context, EnsurerContext, *string, string) error
	// EnsureAdditionalUnits ensures additional systemd units
	// "old" might be "nil" and must always be checked.
	EnsureAdditionalUnits(ctx context.Context, etcx EnsurerContext, new, old *[]extensionsv1alpha1.Unit) error
	// EnsureAdditionalFile ensures additional systemd files
	// "old" might be "nil" and must always be checked.
	EnsureAdditionalFiles(ctx context.Context, etcx EnsurerContext, new, old *[]extensionsv1alpha1.File) error
}

// NewMutator creates a new controlplane mutator.
func NewMutator(
	ensurer Ensurer,
	unitSerializer controlplane.UnitSerializer,
	kubeletConfigCodec controlplane.KubeletConfigCodec,
	fciCodec controlplane.FileContentInlineCodec,
	logger logr.Logger,
) extensionswebhook.Mutator {
	return &mutator{
		ensurer:            ensurer,
		unitSerializer:     unitSerializer,
		kubeletConfigCodec: kubeletConfigCodec,
		fciCodec:           fciCodec,
		logger:             logger.WithName("mutator"),
	}
}

type mutator struct {
	client             client.Client
	ensurer            Ensurer
	unitSerializer     controlplane.UnitSerializer
	kubeletConfigCodec controlplane.KubeletConfigCodec
	fciCodec           controlplane.FileContentInlineCodec
	logger             logr.Logger
}

// InjectClient injects the given client into the ensurer.
// TODO Replace this with the more generic InjectFunc when controller runtime supports it
func (m *mutator) InjectClient(client client.Client) error {
	m.client = client
	if _, err := inject.ClientInto(client, m.ensurer); err != nil {
		return errors.Wrap(err, "could not inject the client into the ensurer")
	}
	return nil
}

type ensurerContext struct {
	client  client.Client
	object  metav1.Object
	cluster *extensionscontroller.Cluster
}

// NewEnsurerContext creates an ensurer context object.
func NewEnsurerContext(client client.Client, object metav1.Object) EnsurerContext {
	return &ensurerContext{
		client: client,
		object: object,
	}
}

// NewInternalEnsurerContext creates an ensurer context object.
func NewInternalEnsurerContext(cluster *extensionscontroller.Cluster) EnsurerContext {
	return &ensurerContext{
		cluster: cluster,
	}
}

// GetCluster returns the cluster object.
func (c *ensurerContext) GetCluster(ctx context.Context) (*extensionscontroller.Cluster, error) {
	if c.cluster == nil {
		cluster, err := extensionscontroller.GetCluster(ctx, c.client, c.object.GetNamespace())
		if err != nil {
			return nil, errors.Wrapf(err, "could not get cluster for namespace '%s'", c.object.GetNamespace())
		}
		c.cluster = cluster
	}
	return c.cluster, nil
}

// Mutate validates and if needed mutates the given object.
func (m *mutator) Mutate(ctx context.Context, new, old runtime.Object) error {
	acc, err := meta.Accessor(new)
	if err != nil {
		return errors.Wrapf(err, "could not create accessor during webhook")
	}
	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if acc.GetDeletionTimestamp() != nil {
		return nil
	}
	o, ok := new.(metav1.Object)
	if !ok {
		return errors.Wrapf(err, "could not cast runtime object to metav1 object")
	}
	ectx := NewEnsurerContext(m.client, o)

	switch x := new.(type) {
	case *corev1.Service:
		switch x.Name {
		case v1beta1constants.DeploymentNameKubeAPIServer:
			var oldSvc *corev1.Service
			if old != nil {
				oldSvc, ok = old.(*corev1.Service)
				if !ok {
					return errors.Wrapf(err, "could not cast old object to corev1.Service")
				}
			}

			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeAPIServerService(ctx, ectx, x, oldSvc)
		}
	case *appsv1.Deployment:
		var oldDep *appsv1.Deployment
		if old != nil {
			oldDep, ok = old.(*appsv1.Deployment)
			if !ok {
				return errors.Wrapf(err, "could not cast old object to appsv1.Deployment")
			}
		}

		switch x.Name {
		case v1beta1constants.DeploymentNameKubeAPIServer:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeAPIServerDeployment(ctx, ectx, x, oldDep)
		case v1beta1constants.DeploymentNameKubeControllerManager:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeControllerManagerDeployment(ctx, ectx, x, oldDep)
		case v1beta1constants.DeploymentNameKubeScheduler:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeSchedulerDeployment(ctx, ectx, x, oldDep)
		}
	case *druidv1alpha1.Etcd:
		switch x.Name {
		case v1beta1constants.ETCDMain, v1beta1constants.ETCDEvents:
			var oldEtcd *druidv1alpha1.Etcd
			if old != nil {
				oldEtcd, ok = old.(*druidv1alpha1.Etcd)
				if !ok {
					return errors.Wrapf(err, "could not cast old object to druidv1alpha1.Etcd")
				}
			}

			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureETCD(ctx, ectx, x, oldEtcd)
		}
	case *extensionsv1alpha1.OperatingSystemConfig:
		if x.Spec.Purpose == extensionsv1alpha1.OperatingSystemConfigPurposeReconcile {
			var oldOSC *extensionsv1alpha1.OperatingSystemConfig
			if old != nil {
				oldOSC, ok = old.(*extensionsv1alpha1.OperatingSystemConfig)
				if !ok {
					return errors.Wrapf(err, "could not cast old object to extensionsv1alpha1.OperatingSystemConfig")
				}
			}

			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.mutateOperatingSystemConfig(ctx, ectx, x, oldOSC)
		}
		return nil
	}
	return nil
}

func getKubeletService(osc *extensionsv1alpha1.OperatingSystemConfig) *string {
	if osc != nil {
		if u := extensionswebhook.UnitWithName(osc.Spec.Units, v1beta1constants.OperatingSystemConfigUnitNameKubeletService); u != nil {
			return u.Content
		}
	}

	return nil
}

func getKubeletConfigFile(osc *extensionsv1alpha1.OperatingSystemConfig) *extensionsv1alpha1.FileContentInline {
	return findFileWithPath(osc, v1beta1constants.OperatingSystemConfigFilePathKubeletConfig)
}

func getKubernetesGeneralConfiguration(osc *extensionsv1alpha1.OperatingSystemConfig) *extensionsv1alpha1.FileContentInline {
	return findFileWithPath(osc, v1beta1constants.OperatingSystemConfigFilePathKernelSettings)
}

func findFileWithPath(osc *extensionsv1alpha1.OperatingSystemConfig, path string) *extensionsv1alpha1.FileContentInline {
	if osc != nil {
		if f := extensionswebhook.FileWithPath(osc.Spec.Files, path); f != nil {
			return f.Content.Inline
		}
	}

	return nil
}

func (m *mutator) mutateOperatingSystemConfig(ctx context.Context, ectx EnsurerContext, osc, oldOSC *extensionsv1alpha1.OperatingSystemConfig) error {
	// Mutate kubelet.service unit, if present
	if content := getKubeletService(osc); content != nil {
		if err := m.ensureKubeletServiceUnitContent(ctx, ectx, content, getKubeletService(oldOSC)); err != nil {
			return err
		}
	}

	// Mutate kubelet configuration file, if present
	if content := getKubeletConfigFile(osc); content != nil {
		if err := m.ensureKubeletConfigFileContent(ctx, ectx, content, getKubeletConfigFile(oldOSC)); err != nil {
			return err
		}
	}

	// Mutate 99 kubernetes general configuration file, if present
	if content := getKubernetesGeneralConfiguration(osc); content != nil {
		if err := m.ensureKubernetesGeneralConfiguration(ctx, ectx, content, getKubernetesGeneralConfiguration(oldOSC)); err != nil {
			return err
		}
	}

	// Check if cloud provider config needs to be ensured
	if m.ensurer.ShouldProvisionKubeletCloudProviderConfig(ctx, ectx) {
		if err := m.ensureKubeletCloudProviderConfig(ctx, ectx, osc); err != nil {
			return err
		}
	}

	var (
		oldFiles *[]extensionsv1alpha1.File
		oldUnits *[]extensionsv1alpha1.Unit
	)

	if oldOSC != nil {
		oldFiles = &oldOSC.Spec.Files
		oldUnits = &oldOSC.Spec.Units
	}

	if err := m.ensurer.EnsureAdditionalFiles(ctx, ectx, &osc.Spec.Files, oldFiles); err != nil {
		return err
	}

	if err := m.ensurer.EnsureAdditionalUnits(ctx, ectx, &osc.Spec.Units, oldUnits); err != nil {
		return err
	}

	return nil
}

func (m *mutator) ensureKubeletServiceUnitContent(ctx context.Context, ectx EnsurerContext, content, oldContent *string) error {
	var (
		opts, oldOpts []*unit.UnitOption
		err           error
	)

	// Deserialize unit options
	if opts, err = m.unitSerializer.Deserialize(*content); err != nil {
		return errors.Wrap(err, "could not deserialize kubelet.service unit content")
	}

	if oldContent != nil {
		// Deserialize old unit options
		if oldOpts, err = m.unitSerializer.Deserialize(*oldContent); err != nil {
			return errors.Wrap(err, "could not deserialize old kubelet.service unit content")
		}
	}

	if opts, err = m.ensurer.EnsureKubeletServiceUnitOptions(ctx, ectx, opts, oldOpts); err != nil {
		return err
	}

	// Serialize unit options
	if *content, err = m.unitSerializer.Serialize(opts); err != nil {
		return errors.Wrap(err, "could not serialize kubelet.service unit options")
	}

	return nil
}

func (m *mutator) ensureKubeletConfigFileContent(ctx context.Context, ectx EnsurerContext, fci, oldFCI *extensionsv1alpha1.FileContentInline) error {
	var (
		kubeletConfig, oldKubeletConfig *kubeletconfigv1beta1.KubeletConfiguration
		err                             error
	)

	// Decode kubelet configuration from inline content
	if kubeletConfig, err = m.kubeletConfigCodec.Decode(fci); err != nil {
		return errors.Wrap(err, "could not decode kubelet configuration")
	}

	if oldFCI != nil {
		// Decode old kubelet configuration from inline content
		if oldKubeletConfig, err = m.kubeletConfigCodec.Decode(oldFCI); err != nil {
			return errors.Wrap(err, "could not decode old kubelet configuration")
		}
	}

	if err = m.ensurer.EnsureKubeletConfiguration(ctx, ectx, kubeletConfig, oldKubeletConfig); err != nil {
		return err
	}

	// Encode kubelet configuration into inline content
	var newFCI *extensionsv1alpha1.FileContentInline
	if newFCI, err = m.kubeletConfigCodec.Encode(kubeletConfig, fci.Encoding); err != nil {
		return errors.Wrap(err, "could not encode kubelet configuration")
	}
	*fci = *newFCI

	return nil
}

func (m *mutator) ensureKubernetesGeneralConfiguration(ctx context.Context, ectx EnsurerContext, fci, oldFCI *extensionsv1alpha1.FileContentInline) error {
	var (
		data, oldData []byte
		err           error
	)

	// Decode kubernetes general configuration from inline content
	if data, err = m.fciCodec.Decode(fci); err != nil {
		return errors.Wrap(err, "could not decode kubernetes general configuration")
	}

	if oldFCI != nil {
		// Decode kubernetes general configuration from inline content
		if oldData, err = m.fciCodec.Decode(oldFCI); err != nil {
			return errors.Wrap(err, "could not decode old kubernetes general configuration")
		}
	}

	s := string(data)
	oldS := string(oldData)
	if err = m.ensurer.EnsureKubernetesGeneralConfiguration(ctx, ectx, &s, &oldS); err != nil {
		return err
	}

	// Encode kubernetes general configuration into inline content
	var newFCI *extensionsv1alpha1.FileContentInline
	if newFCI, err = m.fciCodec.Encode([]byte(s), fci.Encoding); err != nil {
		return errors.Wrap(err, "could not encode kubernetes general configuration")
	}
	*fci = *newFCI

	return nil
}

const CloudProviderConfigPath = "/var/lib/kubelet/cloudprovider.conf"

func (m *mutator) ensureKubeletCloudProviderConfig(ctx context.Context, ectx EnsurerContext, osc *extensionsv1alpha1.OperatingSystemConfig) error {
	var err error

	// Ensure kubelet cloud provider config
	var s string
	if err = m.ensurer.EnsureKubeletCloudProviderConfig(ctx, ectx, &s, osc.Namespace); err != nil {
		return err
	}

	// Encode cloud provider config into inline content
	var fci *extensionsv1alpha1.FileContentInline
	if fci, err = m.fciCodec.Encode([]byte(s), string(cloudinit.B64FileCodecID)); err != nil {
		return errors.Wrap(err, "could not encode kubelet cloud provider config")
	}

	// Ensure the cloud provider config file is part of the OperatingSystemConfig
	osc.Spec.Files = extensionswebhook.EnsureFileWithPath(osc.Spec.Files, extensionsv1alpha1.File{
		Path:        CloudProviderConfigPath,
		Permissions: pointer.Int32Ptr(0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: fci,
		},
	})
	return nil
}
