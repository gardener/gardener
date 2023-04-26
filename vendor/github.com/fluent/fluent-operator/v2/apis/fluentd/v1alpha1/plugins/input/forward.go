package input

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/common"
)

// Forward defines the in_forward Input plugin that listens to a TCP socket to receive the event stream.
type Forward struct {
	// The transport section of forward plugin
	Transport *common.Transport `json:"transport,omitempty"`
	// The security section of forward plugin
	Security *common.Security `json:"security,omitempty"`
	// The security section of client plugin
	Client *common.Client `json:"client,omitempty"`
	// The security section of user plugin
	User *common.User `json:"user,omitempty"`
	// The port to listen to, default is 24224.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=65535
	Port *int32 `json:"port,omitempty"`
	// The port to listen to, default is "0.0.0.0"
	Bind *string `json:"bind,omitempty"`
	// in_forward uses incoming event's tag by default (See Protocol Section).
	// If the tag parameter is set, its value is used instead.
	Tag *string `json:"tag,omitempty"`
	// Adds the prefix to the incoming event's tag.
	AddTagPrefix *string `json:"addTagPrefix,omitempty"`
	// The timeout used to set the linger option.
	LingerTimeout *uint16 `json:"lingerTimeout,omitempty"`
	// Tries to resolve hostname from IP addresses or not.
	ResolveHostname *bool `json:"resolveHostname,omitempty"`
	// The connections will be disconnected right after receiving a message, if true.
	DenyKeepalive *bool `json:"denyKeepalive,omitempty"`
	// Enables the TCP keepalive for sockets.
	SendKeepalivePacket *bool `json:"sendKeepalivePacket,omitempty"`
	// The size limit of the received chunk. If the chunk size is larger than this value, the received chunk is dropped.
	// +kubebuilder:validation:Pattern:="^\\d+(KB|MB|GB|TB)$"
	ChunkSizeLimit *string `json:"chunkSizeLimit,omitempty"`
	// The warning size limit of the received chunk. If the chunk size is larger than this value, a warning message will be sent.
	// +kubebuilder:validation:Pattern:="^\\d+(KB|MB|GB|TB)$"
	ChunkSizeWarnLimit *string `json:"chunkSizeWarnLimit,omitempty"`
	// Skips the invalid incoming event.
	SkipInvalidEvent *bool `json:"skipInvalidEvent,omitempty"`
	// The field name of the client's source address. If set, the client's address will be set to its key.
	SourceAddressKey *string `json:"sourceAddressKey,omitempty"`
	// The field name of the client's hostname. If set, the client's hostname will be set to its key.
	SourceHostnameKey *string `json:"sourceHostnameKey,omitempty"`
}
