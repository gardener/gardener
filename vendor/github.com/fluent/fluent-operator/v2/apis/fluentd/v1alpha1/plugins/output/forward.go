package output

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/common"
)

// Forward defines the out_forward Buffered Output plugin forwards events to other fluentd nodes.
type Forward struct {
	// Servers defines the servers section, at least one is required
	Servers []*common.Server `json:"servers"`
	// ServiceDiscovery defines the service_discovery section
	ServiceDiscovery *common.ServiceDiscovery `json:"serviceDiscovery,omitempty"`
	// ServiceDiscovery defines the security section
	Security *common.Security `json:"security,omitempty"`
	// Changes the protocol to at-least-once. The plugin waits the ack from destination's in_forward plugin.
	RequireAckResponse *bool `json:"requireAckResponse,omitempty"`
	// This option is used when require_ack_response is true. This default value is based on popular tcp_syn_retries.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	AckResponseTimeout *string `json:"ackResponseTimeout,omitempty"`
	// The timeout time when sending event logs.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	SendTimeout *string `json:"sendTimeout,omitempty"`
	// The connection timeout for the socket. When the connection is timed out during the connection establishment, Errno::ETIMEDOUT error is raised.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	ConnectTimeout *string `json:"connectTimeout,omitempty"`
	// The wait time before accepting a server fault recovery.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	RecoverWait *string `json:"recoverWait,omitempty"`
	// Specifies the transport protocol for heartbeats. Set none to disable.
	// +kubebuilder:validation:Enum:=transport;tcp;udp;none
	HeartbeatType *string `json:"heartbeatType,omitempty"`
	// The interval of the heartbeat packer.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	HeartbeatInterval *string `json:"heartbeatInterval,omitempty"`
	// Use the "Phi accrual failure detector" to detect server failure.
	PhiFailureDetector *bool `json:"phiFailureDetector,omitempty"`
	// The threshold parameter used to detect server faults.
	PhiThreshold *uint16 `json:"phiThreshold,omitempty"`
	// The hard timeout used to detect server failure. The default value is equal to the send_timeout parameter.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	HardTimeout *string `json:"hardTimeout,omitempty"`
	// Sets TTL to expire DNS cache in seconds. Set 0 not to use DNS Cache.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	ExpireDnsCache *string `json:"expireDnsCache,omitempty"`
	// Enable client-side DNS round robin. Uniform randomly pick an IP address to send data when a hostname has several IP addresses.
	// heartbeat_type udp is not available with dns_round_robintrue. Use heartbeat_type tcp or heartbeat_type none.
	DnsRoundRobin *bool `json:"dnsRoundRobin,omitempty"`
	// Ignores DNS resolution and errors at startup time.
	IgnoreNetworkErrorsAtStartup *bool `json:"ignoreNetworkErrorsAtStartup,omitempty"`
	// The default version of TLS transport.
	// +kubebuilder:validation:Enum:=TLSv1_1;TLSv1_2
	TlsVersion *string `json:"tlsVersion,omitempty"`
	// The cipher configuration of TLS transport.
	TlsCiphers *string `json:"tlsCiphers,omitempty"`
	// Skips all verification of certificates or not.
	TlsInsecureMode *bool `json:"tlsInsecureMode,omitempty"`
	// Allows self-signed certificates or not.
	TlsAllowSelfSignedCert *bool `json:"tlsAllowSelfSignedCert,omitempty"`
	// Verifies hostname of servers and certificates or not in TLS transport.
	TlsVerifyHostname *bool `json:"tlsVerifyHostname,omitempty"`
	// The additional CA certificate path for TLS.
	TlsCertPath *string `json:"tlsCertPath,omitempty"`
	// The client certificate path for TLS.
	TlsClientCertPath *string `json:"tlsClientCertPath,omitempty"`
	// The client private key path for TLS.
	TlsClientPrivateKeyPath *string `json:"tlsClientPrivateKeyPath,omitempty"`
	// The TLS private key passphrase for the client.
	TlsClientPrivateKeyPassphrase *string `json:"tlsClientPrivateKeyPassphrase,omitempty"`
	// The certificate thumbprint for searching from Windows system certstore. This parameter is for Windows only.
	TlsCertThumbprint *string `json:"tlsCertThumbprint,omitempty"`
	// The certificate logical store name on Windows system certstore. This parameter is for Windows only.
	TlsCertLogicalStoreName *string `json:"tlsCertLogicalStoreName,omitempty"`
	// Enables the certificate enterprise store on Windows system certstore. This parameter is for Windows only.
	TlsCertUseEnterpriseStore *bool `json:"tlsCertUseEnterpriseStore,omitempty"`
	// Enables the keepalive connection.
	Keepalive *bool `json:"keepalive,omitempty"`
	// Timeout for keepalive. Default value is nil which means to keep the connection alive as long as possible.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	KeepaliveTimeout *string `json:"keepaliveTimeout,omitempty"`
	// Verify that a connection can be made with one of out_forward nodes at the time of startup.
	VerifyConnectionAtStartup *bool `json:"verifyConnectionAtStartup,omitempty"`
}
