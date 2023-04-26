package input

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/common"
)

// Http defines the in_http Input plugin that listens to a TCP socket to receive the event stream.
type Http struct {
	// The transport section of http plugin
	Transport *common.Transport `json:"transport,omitempty"`
	// The parse section of http plugin
	Parse *common.Parse `json:"parse,omitempty"`
	// The port to listen to, default is 9880.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=65535
	Port *int32 `json:"port,omitempty"`
	// The port to listen to, default is "0.0.0.0"
	Bind *string `json:"bind,omitempty"`
	// The size limit of the POSTed element.
	// +kubebuilder:validation:Pattern:="^\\d+(KB|MB|GB|TB)$"
	BodySizeLimit *string `json:"bodySizeLimit,omitempty"`
	// The timeout limit for keeping the connection alive.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	KeepLiveTimeout *string `json:"keepaliveTimeout,omitempty"`
	// Adds HTTP_ prefix headers to the record.
	AddHttpHeaders *bool `json:"addHttpHeaders,omitempty"`
	// Adds REMOTE_ADDR field to the record. The value of REMOTE_ADDR is the client's address.
	// i.e: X-Forwarded-For: host1, host2
	AddRemoteAddr *string `json:"addRemoteAddr,omitempty"`
	// Whitelist domains for CORS.
	CorsAllowOrigins *string `json:"corsAllOrigins,omitempty"`
	// Add Access-Control-Allow-Credentials header. It's needed when a request's credentials mode is include
	CorsAllowCredentials *string `json:"corsAllowCredentials,omitempty"`
	// Responds with an empty GIF image of 1x1 pixel (rather than an empty string).
	RespondsWithEmptyImg *bool `json:"respondsWithEmptyImg,omitempty"`
}
