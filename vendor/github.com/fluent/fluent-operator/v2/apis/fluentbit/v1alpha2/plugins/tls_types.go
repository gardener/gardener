package plugins

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// Fluent Bit provides integrated support for Transport Layer Security (TLS) and it predecessor Secure Sockets Layer (SSL) respectively.
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

func (t *TLS) Params(sl SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	kvs.Insert("tls", "On")
	if t.Verify != nil {
		kvs.Insert("tls.verify", fmt.Sprint(*t.Verify))
	}
	if t.Debug != nil {
		kvs.Insert("tls.debug", fmt.Sprint(*t.Debug))
	}
	if t.CAFile != "" {
		kvs.Insert("tls.ca_file", t.CAFile)
	}
	if t.CAPath != "" {
		kvs.Insert("tls.ca_path", t.CAPath)
	}
	if t.CRTFile != "" {
		kvs.Insert("tls.crt_file", t.CRTFile)
	}
	if t.KeyFile != "" {
		kvs.Insert("tls.key_file", t.KeyFile)
	}
	if t.KeyPassword != nil {
		pwd, err := sl.LoadSecret(*t.KeyPassword)
		if err != nil {
			return nil, err
		}
		kvs.Insert("tls.key_passwd", pwd)
	}
	if t.Vhost != "" {
		kvs.Insert("tls.vhost", t.Vhost)
	}
	return kvs, nil
}
