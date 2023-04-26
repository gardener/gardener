package plugins

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Fluentd provides integrated support for Transport Layer Security (TLS) and it predecessor Secure Sockets Layer (SSL) respectively.
type TLS struct {
	// Force certificate validation
	Verify *bool `json:"verify,omitempty"`
	// Set TLS debug verbosity level.
	// It accept the following values: 0 (No debug), 1 (Error), 2 (State change), 3 (Informational) and 4 Verbose
	// +kubebuilder:validation:Enum:=0;1;2;3;4
	Debug *int32 `json:"debug,omitempty"`
	// Absolute path to CA certificate file
	CAFile string `json:"caFile,omitempty"`
	// Absolute path to scan for certificate files
	CAPath string `json:"caPath,omitempty"`
	// Absolute path to Certificate file
	CRTFile string `json:"crtFile,omitempty"`
	// Absolute path to private Key file
	KeyFile string `json:"keyFile,omitempty"`
	// Optional password for tls.key_file file
	KeyPassword *Secret `json:"keyPassword,omitempty"`
	// Hostname to be used for TLS SNI extension
	Vhost string `json:"vhost,omitempty"`
}

type TLSLoader struct {
	client    client.Client
	namespace string
}

func NewTLSMapLoader(c client.Client, ns string) TLSLoader {
	return TLSLoader{
		client:    c,
		namespace: ns,
	}
}
