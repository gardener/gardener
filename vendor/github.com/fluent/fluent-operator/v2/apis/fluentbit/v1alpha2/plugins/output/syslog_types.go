package output

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// Syslog output plugin allows you to deliver messages to Syslog servers. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/syslog**
type Syslog struct {
	// Host domain or IP address of the remote Syslog server.
	Host string `json:"host,omitempty"`
	// TCP or UDP port of the remote Syslog server.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=65535
	Port *int32 `json:"port,omitempty"`
	// Mode of the desired transport type, the available options are tcp, tls and udp.
	Mode string `json:"mode,omitempty"`
	// Syslog protocol format to use, the available options are rfc3164 and rfc5424.
	SyslogFormat string `json:"syslogFormat,omitempty"`
	// Maximum size allowed per message, in bytes.
	SyslogMaxSize *int32 `json:"syslogMaxSize,omitempty"`
	// Key from the original record that contains the Syslog severity number.
	SyslogSeverityKey string `json:"syslogSeverityKey,omitempty"`
	// Key from the original record that contains the Syslog facility number.
	SyslogFacilityKey string `json:"syslogFacilityKey,omitempty"`
	// Key name from the original record that contains the hostname that generated the message.
	SyslogHostnameKey string `json:"syslogHostnameKey,omitempty"`
	// Key name from the original record that contains the application name that generated the message.
	SyslogAppnameKey string `json:"syslogAppnameKey,omitempty"`
	// Key name from the original record that contains the Process ID that generated the message.
	SyslogProcessIDKey string `json:"syslogProcessIDKey,omitempty"`
	// Key name from the original record that contains the Message ID associated to the message.
	SyslogMessageIDKey string `json:"syslogMessageIDKey,omitempty"`
	// Key name from the original record that contains the Structured Data (SD) content.
	SyslogSDKey string `json:"syslogSDKey,omitempty"`
	// Key key name that contains the message to deliver.
	SyslogMessageKey string `json:"syslogMessageKey,omitempty"`
	// Syslog output plugin supports TTL/SSL, for more details about the properties available
	// and general configuration, please refer to the TLS/SSL section.
	*plugins.TLS `json:"tls,omitempty"`
}

func (_ *Syslog) Name() string {
	return "syslog"
}

// implement Section() method
func (s *Syslog) Params(sl plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if s.Host != "" {
		kvs.Insert("Host", s.Host)
	}
	if s.Port != nil {
		kvs.Insert("port", fmt.Sprint(*s.Port))
	}
	if s.Mode != "" {
		kvs.Insert("mode", s.Mode)
	}
	if s.SyslogFormat != "" {
		kvs.Insert("syslog_format", s.SyslogFormat)
	}
	if s.SyslogMaxSize != nil {
		kvs.Insert("syslog_maxsize", fmt.Sprint(s.SyslogMaxSize))
	}
	if s.SyslogSeverityKey != "" {
		kvs.Insert("syslog_severity_key", s.SyslogSeverityKey)
	}
	if s.SyslogFacilityKey != "" {
		kvs.Insert("syslog_facility_key", s.SyslogFacilityKey)
	}
	if s.SyslogHostnameKey != "" {
		kvs.Insert("syslog_hostname_key", s.SyslogHostnameKey)
	}
	if s.SyslogAppnameKey != "" {
		kvs.Insert("syslog_appname_key", s.SyslogAppnameKey)
	}
	if s.SyslogProcessIDKey != "" {
		kvs.Insert("syslog_procid_key", s.SyslogProcessIDKey)
	}
	if s.SyslogMessageIDKey != "" {
		kvs.Insert("syslog_msgid_key", s.SyslogMessageIDKey)
	}
	if s.SyslogSDKey != "" {
		kvs.Insert("syslog_sd_key", s.SyslogSDKey)
	}
	if s.SyslogMessageKey != "" {
		kvs.Insert("syslog_message_key", s.SyslogMessageKey)
	}
	if s.TLS != nil {
		tls, err := s.TLS.Params(sl)
		if err != nil {
			return nil, err
		}
		kvs.Merge(tls)
	}
	return kvs, nil
}
