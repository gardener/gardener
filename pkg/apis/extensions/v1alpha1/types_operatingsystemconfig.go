// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// ReloadConfigFilePath is the path to the generated operating system configuration. If set, controllers
	// are asked to use it when determining the .status.command of this resource. For example, if for CoreOS
	// the reload-path might be "/var/lib/config"; then the controller shall set .status.command to
	// "/usr/bin/coreos-cloudinit --from-file=/var/lib/config".
	// Deprecated: This field is deprecated and has no further usage.
	// TODO(rfranzke): Remove this field after v1.95 got released.
	// +optional
	ReloadConfigFilePath *string `json:"reloadConfigFilePath,omitempty"`
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
	// Should be defaulted to octal 0644.
	// +optional
	Permissions *int32 `json:"permissions,omitempty"`
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
	// +optional
	CloudConfig *CloudConfig `json:"cloudConfig,omitempty"`
	// Command is the command whose execution renews/reloads the cloud config on an existing VM, e.g.
	// "/usr/bin/reload-cloud-config -from-file=<path>". The <path> is optionally provided by Gardener
	// in the .spec.reloadConfigFilePath field.
	// Deprecated: This field is deprecated and has no further usage.
	// TODO(rfranzke): Remove this field after v1.95 got released.
	// +optional
	Command *string `json:"command,omitempty"`
	// Units is a list of systemd unit names that are part of the generated Cloud Config and shall be
	// restarted when a new version has been downloaded.
	// Deprecated: This field is deprecated and has no further usage.
	// TODO(rfranzke): Remove this field after v1.95 got released.
	// +optional
	Units []string `json:"units,omitempty"`
	// Files is a list of file paths that are part of the generated Cloud Config and shall be
	// written to the host's file system.
	// Deprecated: This field is deprecated and has no further usage.
	// TODO(rfranzke): Remove this field after v1.95 got released.
	// +optional
	Files []string `json:"files,omitempty"`
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

	// OperatingSystemConfigDefaultFilePermission is the default value for a permission of a file.
	OperatingSystemConfigDefaultFilePermission int32 = 0644
	// OperatingSystemConfigSecretDataKey is a constant for the key in a secret's `.data` field containing the
	// results of a computed cloud config.
	OperatingSystemConfigSecretDataKey = "cloud_config"
)

// CRIConfig contains configurations of the CRI library.
type CRIConfig struct {
	// Name is a mandatory string containing the name of the CRI library. Supported values are `containerd`.
	Name CRIName `json:"name"`
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
