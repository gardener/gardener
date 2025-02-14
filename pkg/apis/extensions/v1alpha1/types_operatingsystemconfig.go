// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ Object = (*OperatingSystemConfig)(nil)

// OperatingSystemConfigResource is a constant for the name of the OperatingSystemConfig resource.
const OperatingSystemConfigResource = "OperatingSystemConfig"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,path=operatingsystemconfigs,shortName=osc,singular=operatingsystemconfig
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Type,JSONPath=".spec.type",type=string,description="The type of the operating system configuration."
// +kubebuilder:printcolumn:name=Purpose,JSONPath=".spec.purpose",type=string,description="The purpose of the operating system configuration."
// +kubebuilder:printcolumn:name=Status,JSONPath=".status.lastOperation.state",type=string,description="Status of operating system configuration."
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date,description="creation timestamp"

// OperatingSystemConfig is a specification for a OperatingSystemConfig resource
type OperatingSystemConfig struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the OperatingSystemConfig.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec OperatingSystemConfigSpec `json:"spec"`
	// +optional
	Status OperatingSystemConfigStatus `json:"status"`
}

// GetExtensionSpec implements Object.
func (o *OperatingSystemConfig) GetExtensionSpec() Spec {
	return &o.Spec
}

// GetExtensionPurpose implements Object.
func (o *OperatingSystemConfigSpec) GetExtensionPurpose() *string {
	return (*string)(&o.Purpose)
}

// GetExtensionStatus implements Object.
func (o *OperatingSystemConfig) GetExtensionStatus() Status {
	return &o.Status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperatingSystemConfigList is a list of OperatingSystemConfig resources.
type OperatingSystemConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of OperatingSystemConfigs.
	Items []OperatingSystemConfig `json:"items"`
}

// OperatingSystemConfigSpec is the spec for a OperatingSystemConfig resource.
type OperatingSystemConfigSpec struct {
	// CRI config is a structure contains configurations of the CRI library
	// +optional
	CRIConfig *CRIConfig `json:"criConfig,omitempty"`
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`
	// Purpose describes how the result of this OperatingSystemConfig is used by Gardener. Either it
	// gets sent to the `Worker` extension controller to bootstrap a VM, or it is downloaded by the
	// gardener-node-agent already running on a bootstrapped VM.
	// This field is immutable.
	Purpose OperatingSystemConfigPurpose `json:"purpose"`
	// Units is a list of unit for the operating system configuration (usually, a systemd unit).
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Units []Unit `json:"units,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
	// Files is a list of files that should get written to the host's file system.
	// +patchMergeKey=path
	// +patchStrategy=merge
	// +optional
	Files []File `json:"files,omitempty" patchStrategy:"merge" patchMergeKey:"path"`
	// InPlaceUpdates contains the configuration for in-place updates.
	// +optional
	InPlaceUpdates *InPlaceUpdates `json:"inPlaceUpdates,omitempty"`
}

// Unit is a unit for the operating system configuration (usually, a systemd unit).
type Unit struct {
	// Name is the name of a unit.
	Name string `json:"name"`
	// Command is the unit's command.
	// +optional
	Command *UnitCommand `json:"command,omitempty"`
	// Enable describes whether the unit is enabled or not.
	// +optional
	Enable *bool `json:"enable,omitempty"`
	// Content is the unit's content.
	// +optional
	Content *string `json:"content,omitempty"`
	// DropIns is a list of drop-ins for this unit.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	DropIns []DropIn `json:"dropIns,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
	// FilePaths is a list of files the unit depends on. If any file changes a restart of the dependent unit will be
	// triggered. For each FilePath there must exist a File with matching Path in OperatingSystemConfig.Spec.Files.
	FilePaths []string `json:"filePaths,omitempty"`
}

// UnitCommand is a string alias.
type UnitCommand string

const (
	// CommandStart is the 'start' command for a unit.
	CommandStart UnitCommand = "start"
	// CommandRestart is the 'restart' command for a unit.
	CommandRestart UnitCommand = "restart"
	// CommandStop is the 'stop' command for a unit.
	CommandStop UnitCommand = "stop"
)

// DropIn is a drop-in configuration for a systemd unit.
type DropIn struct {
	// Name is the name of the drop-in.
	Name string `json:"name"`
	// Content is the content of the drop-in.
	Content string `json:"content"`
}

// File is a file that should get written to the host's file system. The content can either be inlined or
// referenced from a secret in the same namespace.
type File struct {
	// Path is the path of the file system where the file should get written to.
	Path string `json:"path"`
	// Permissions describes with which permissions the file should get written to the file system.
	// If no permissions are set, the operating system's defaults are used.
	// +optional
	Permissions *uint32 `json:"permissions,omitempty"`
	// Content describe the file's content.
	Content FileContent `json:"content"`
}

// FileContent can either reference a secret or contain inline configuration.
type FileContent struct {
	// SecretRef is a struct that contains information about the referenced secret.
	// +optional
	SecretRef *FileContentSecretRef `json:"secretRef,omitempty"`
	// Inline is a struct that contains information about the inlined data.
	// +optional
	Inline *FileContentInline `json:"inline,omitempty"`
	// TransmitUnencoded set to true will ensure that the os-extension does not encode the file content when sent to the node.
	// This for example can be used to manipulate the clear-text content before it reaches the node.
	// +optional
	TransmitUnencoded *bool `json:"transmitUnencoded,omitempty"`
	// ImageRef describes a container image which contains a file.
	// +optional
	ImageRef *FileContentImageRef `json:"imageRef,omitempty"`
}

// FileContentSecretRef contains keys for referencing a file content's data from a secret in the same namespace.
type FileContentSecretRef struct {
	// Name is the name of the secret.
	Name string `json:"name"`
	// DataKey is the key in the secret's `.data` field that should be read.
	DataKey string `json:"dataKey"`
}

// FileContentInline contains keys for inlining a file content's data and encoding.
type FileContentInline struct {
	// Encoding is the file's encoding (e.g. base64).
	Encoding string `json:"encoding"`
	// Data is the file's data.
	Data string `json:"data"`
}

// FileContentImageRef describes a container image which contains a file
type FileContentImageRef struct {
	// Image contains the container image repository with tag.
	Image string `json:"image"`
	// FilePathInImage contains the path in the image to the file that should be extracted.
	FilePathInImage string `json:"filePathInImage"`
}

// OperatingSystemConfigStatus is the status for a OperatingSystemConfig resource.
type OperatingSystemConfigStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`
	// ExtensionUnits is a list of additional systemd units provided by the extension.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	ExtensionUnits []Unit `json:"extensionUnits,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
	// ExtensionFiles is a list of additional files provided by the extension.
	// +patchMergeKey=path
	// +patchStrategy=merge
	// +optional
	ExtensionFiles []File `json:"extensionFiles,omitempty" patchStrategy:"merge" patchMergeKey:"path"`
	// CloudConfig is a structure for containing the generated output for the given operating system
	// config spec. It contains a reference to a secret as the result may contain confidential data.
	// After Gardener v1.112, this will be only set for OperatingSystemConfigs with purpose 'provision'.
	// +optional
	CloudConfig *CloudConfig `json:"cloudConfig,omitempty"`
	// InPlaceUpdates contains the configuration for in-place updates.
	// +optional
	InPlaceUpdates *InPlaceUpdatesStatus `json:"inPlaceUpdates,omitempty"`
}

// CloudConfig contains the generated output for the given operating system
// config spec. It contains a reference to a secret as the result may contain confidential data.
type CloudConfig struct {
	// SecretRef is a reference to a secret that contains the actual result of the generated cloud config.
	SecretRef corev1.SecretReference `json:"secretRef"`
}

// OperatingSystemConfigPurpose is a string alias.
type OperatingSystemConfigPurpose string

const (
	// OperatingSystemConfigPurposeProvision describes that the operating system configuration is used to bootstrap a
	// new VM.
	OperatingSystemConfigPurposeProvision OperatingSystemConfigPurpose = "provision"
	// OperatingSystemConfigPurposeReconcile describes that the operating system configuration is executed on an already
	// provisioned VM by the gardener-node-agent.
	OperatingSystemConfigPurposeReconcile OperatingSystemConfigPurpose = "reconcile"

	// OperatingSystemConfigSecretDataKey is a constant for the key in a secret's `.data` field containing the
	// results of a computed cloud config.
	OperatingSystemConfigSecretDataKey = "cloud_config" // #nosec G101 -- No credential.
)

// CgroupDriverName is a string denoting the CRI cgroup driver.
type CgroupDriverName string

const (
	// CgroupDriverCgroupfs is the name of the 'cgroupfs' cgroup driver.
	CgroupDriverCgroupfs CgroupDriverName = "cgroupfs"
	// CgroupDriverSystemd is the name of the 'systemd' cgroup driver.
	CgroupDriverSystemd CgroupDriverName = "systemd"
)

// CRIConfig contains configurations of the CRI library.
type CRIConfig struct {
	// Name is a mandatory string containing the name of the CRI library. Supported values are `containerd`.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:validation:Enum="containerd"
	Name CRIName `json:"name"`
	// CgroupDriver configures the CRI's cgroup driver. Supported values are `cgroupfs` or `systemd`.
	// +optional
	CgroupDriver *CgroupDriverName `json:"cgroupDriver,omitempty"`
	// ContainerdConfig is the containerd configuration.
	// Only to be set for OperatingSystemConfigs with purpose 'reconcile'.
	// +optional
	Containerd *ContainerdConfig `json:"containerd,omitempty"`
}

// ContainerdConfig contains configuration options for containerd.
type ContainerdConfig struct {
	// Registries configures the registry hosts for containerd.
	// +optional
	Registries []RegistryConfig `json:"registries,omitempty"`
	// SandboxImage configures the sandbox image for containerd.
	SandboxImage string `json:"sandboxImage"`
	// Plugins configures the plugins section in containerd's config.toml.
	// +optional
	Plugins []PluginConfig `json:"plugins,omitempty"`
}

// PluginPathOperation is a type alias for operations at containerd's plugin configuration.
type PluginPathOperation string

const (
	// AddPluginPathOperation is the name of the 'add' operation.
	AddPluginPathOperation PluginPathOperation = "add"
	// RemovePluginPathOperation is the name of the 'remove' operation.
	RemovePluginPathOperation PluginPathOperation = "remove"
)

// PluginConfig contains configuration values for the containerd plugins section.
type PluginConfig struct {
	// Op is the operation for the given path. Possible values are 'add' and 'remove', defaults to 'add'.
	// +optional
	Op *PluginPathOperation `json:"op,omitempty"`
	// Path is a list of elements that construct the path in the plugins section.
	Path []string `json:"path"`
	// Values are the values configured at the given path. If defined, it is expected as json format:
	// - A given json object will be put to the given path.
	// - If not configured, only the table entry to be created.
	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`
}

// RegistryConfig contains registry configuration options.
type RegistryConfig struct {
	// Upstream is the upstream name of the registry.
	Upstream string `json:"upstream"`
	// Server is the URL to registry server of this upstream.
	// It corresponds to the server field in the `hosts.toml` file, see https://github.com/containerd/containerd/blob/c51463010e0682f76dfdc10edc095e6596e2764b/docs/hosts.md#server-field for more information.
	// +optional
	Server *string `json:"server,omitempty"`
	// Hosts are the registry hosts.
	// It corresponds to the host fields in the `hosts.toml` file, see https://github.com/containerd/containerd/blob/c51463010e0682f76dfdc10edc095e6596e2764b/docs/hosts.md#host-fields-in-the-toml-table-format for more information.
	Hosts []RegistryHost `json:"hosts,omitempty"`
	// ReadinessProbe determines if host registry endpoints should be probed before they are added to the containerd config.
	// +optional
	ReadinessProbe *bool `json:"readinessProbe,omitempty"`
}

// RegistryCapability specifies an action a client can perform against a registry.
type RegistryCapability string

const (
	// PullCapability defines the 'pull' capability.
	PullCapability RegistryCapability = "pull"
	// ResolveCapability defines the 'resolve' capability.
	ResolveCapability RegistryCapability = "resolve"
	// PushCapability defines the 'push' capability.
	PushCapability RegistryCapability = "push"
)

// RegistryHost contains configuration values for a registry host.
type RegistryHost struct {
	// URL is the endpoint address of the registry mirror.
	URL string `json:"url"`
	// Capabilities determine what operations a host is
	// capable of performing. Defaults to
	//  - pull
	//  - resolve
	Capabilities []RegistryCapability `json:"capabilities,omitempty"`
	// CACerts are paths to public key certificates used for TLS.
	CACerts []string `json:"caCerts,omitempty"`
}

// CRIName is a type alias for the CRI name string.
type CRIName string

const (
	// CRINameContainerD is a constant for ContainerD CRI name
	CRINameContainerD CRIName = "containerd"
)

// ContainerDRuntimeContainersBinFolder is the folder where Container Runtime binaries should be saved for ContainerD usage
const ContainerDRuntimeContainersBinFolder = "/var/bin/containerruntimes"

// FileCodecID is the id of a FileCodec for cloud-init scripts.
type FileCodecID string

const (
	// PlainFileCodecID is the plain file codec id.
	PlainFileCodecID FileCodecID = ""
	// B64FileCodecID is the base64 file codec id.
	B64FileCodecID FileCodecID = "b64"
)

// InPlaceUpdates is a structure containing configuration for in-place updates.
type InPlaceUpdates struct {
	// OperatingSystemVersion is the version of the operating system.
	OperatingSystemVersion string `json:"operatingSystemVersion"`
	// Kubelet contains the configuration for the kubelet.
	Kubelet KubeletConfig `json:"kubelet"`
	// CredentialsRotation is a structure containing information about the last initiation time of the certificate authority and service account key rotation.
	// +optional
	CredentialsRotation *CredentialsRotation `json:"credentialsRotation,omitempty"`
}

type KubeletConfig struct {
	// Version is the version of the kubelet.
	Version string `json:"version"`
	// CPUManagerPolicy allows to set alternative CPU management policies (default: none).
	// +optional
	CPUManagerPolicy *string `json:"cpuManagerPolicy,omitempty"`
	// EvictionHard describes a set of eviction thresholds (e.g. memory.available<1Gi) that if met would trigger a Pod eviction.
	// +optional
	EvictionHard *gardencorev1beta1.KubeletConfigEviction `json:"evictionHard,omitempty"`
	// KubeReserved is the configuration for resources reserved for kubernetes node components (mainly kubelet and container runtime).
	// +optional
	KubeReserved *gardencorev1beta1.KubeletConfigReserved `json:"kubeReserved,omitempty"`
	// SystemReserved is the configuration for resources reserved for system processes not managed by kubernetes (e.g. journald).
	// +optional
	SystemReserved *gardencorev1beta1.KubeletConfigReserved `json:"systemReserved,omitempty"`
}

// InPlaceUpdates is a structure containing configuration for in-place updates.
type InPlaceUpdatesStatus struct {
	// OSUpdate defines the configuration for the operating system update.
	// +optional
	OSUpdate *OSUpdate `json:"osUpdate,omitempty"`
}

// OSUpdate contains the configuration for the operating system update.
type OSUpdate struct {
	// Command defines the command responsible for performing machine image updates.
	Command string `json:"command"`
	// Args provides a mechanism to pass additional arguments or flags to the Command.
	// +optional
	Args []string `json:"args,omitempty"`
}

// CredentialsRotation is a structure containing information about the last initiation time of the certificate authority and service account key rotation.
type CredentialsRotation struct {
	// CertificateAuthorities contains information about the certificate authority credential rotation.
	// +optional
	CertificateAuthorities *CARotation `json:"certificateAuthorities,omitempty"`
	// ServiceAccountKey contains information about the service account key credential rotation.
	// +optional
	ServiceAccountKey *ServiceAccountKeyRotation `json:"serviceAccountKey,omitempty"`
}

// CARotation contains information about the certificate authority credential rotation.
type CARotation struct {
	// LastInitiationTime is the most recent time when the certificate authority credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty"`
}

// ServiceAccountKeyRotation contains information about the service account key credential rotation.
type ServiceAccountKeyRotation struct {
	// LastInitiationTime is the most recent time when the service account key credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty"`
}
