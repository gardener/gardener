package common

import (
	"fmt"

	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins"
	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins/params"
)

// +kubebuilder:object:generate:=true
// CommonFields defines the common parameters for all plugins
type CommonFields struct {
	// The @id parameter specifies a unique name for the configuration.
	Id *string `json:"id,omitempty"`
	// The @type parameter specifies the type of the plugin.
	Type *string `json:"type,omitempty"`
	// The @log_level parameter specifies the plugin-specific logging level
	LogLevel *string `json:"logLevel,omitempty"`
}

// Time defines the common parameters for the time plugin
type Time struct {
	// parses/formats value according to this type, default is *string
	// +kubebuilder:validation:Enum:=float;unixtime;*string;mixed
	TimeType *string `json:"timeType,omitempty"`
	// Process value according to the specified format. This is available only when time_type is *string
	TimeFormat *string `json:"timeFormat,omitempty"`
	// If true, uses local time.
	Localtime *bool `json:"localtime,omitempty"`
	// If true, uses UTC.
	UTC *bool `json:"utc,omitempty"`
	// Uses the specified timezone.
	Timezone *string `json:"timezone,omitempty"`
	// Uses the specified time format as a fallback in the specified order. You can parse undetermined time format by using time_format_fallbacks. This options is enabled when time_type is mixed.
	TimeFormatFallbacks *string `json:"timeFormatFallbacks,omitempty"`
}

// Inject defines the common parameters for the inject plugin
// The inject section can be under <match> or <filter> section.
type Inject struct {
	// Time section
	Time `json:"inline,omitempty"`

	// The field name to inject hostname
	HostnameKey *string `json:"hostnameKey,omitempty"`
	// Hostname value
	Hostname *string `json:"hostname,omitempty"`
	// The field name to inject worker_id
	WorkerIdKey *string `json:"workerIdKey,omitempty"`
	// The field name to inject tag
	TagKey *string `json:"tagKey,omitempty"`
	// The field name to inject time
	TimeKey *string `json:"timeKey,omitempty"`
}

// Security defines the common parameters for the security plugin
type Security struct {
	// The hostname.
	SelfHostname *string `json:"selfHostname,omitempty"`
	// The shared key for authentication.
	SharedKey *string `json:"sharedKey,omitempty"`
	// If true, user-based authentication is used.
	UserAuth *string `json:"userAuth,omitempty"`
	// Allows the anonymous source. <client> sections are required, if disabled.
	AllowAnonymousSource *string `json:"allowAnonymousSource,omitempty"`
	// Defines user section directly.
	*User `json:"user,omitempty"`
}

// User defines the common parameters for the user plugin
type User struct {
	Username *plugins.Secret `json:"username,omitempty"`
	Password *plugins.Secret `json:"password,omitempty"`
}

// Transport defines the commont parameters for the transport plugin
type Transport struct {
	// The protocal name of this plugin, i.e: tls
	Protocol *string `json:"protocol,omitempty"`

	Version  *string `json:"version,omitempty"`
	Ciphers  *string `json:"ciphers,omitempty"`
	Insecure *bool   `json:"insecure,omitempty"`

	// for Cert signed by public CA
	CaPath               *string `json:"caPath,omitempty"`
	CertPath             *string `json:"certPath,omitempty"`
	PrivateKeyPath       *string `json:"privateKeyPath,omitempty"`
	PrivateKeyPassphrase *string `json:"privateKeyPassphrase,omitempty"`
	ClientCertAuth       *bool   `json:"clientCertAuth,omitempty"`

	// for Cert generated
	CaCertPath             *string `json:"caCertPath,omitempty"`
	CaPrivateKeyPath       *string `json:"caPrivateKeyPath,omitempty"`
	CaPrivateKeyPassphrase *string `json:"caPrivateKeyPassphrase,omitempty"`

	// other parameters
	CertVerifier *string `json:"certVerifier,omitempty"`
}

// Client defines the commont parameters for the client plugin
type Client struct {
	// The IP address or hostname of the client. This is exclusive with Network.
	Host *string `json:"host,omitempty"`
	// The network address specification. This is exclusive with Host.
	Network *string `json:"network,omitempty"`
	// The shared key per client.
	SharedKey *string `json:"sharedKey,omitempty"`
	// The array of usernames.
	Users *string `json:"users,omitempty"`
}

// Auth defines the common parameters for the auth plugin
type Auth struct {
	// The method for HTTP authentication. Now only basic.
	// +kubebuilder:validation:Enum:basic
	Method *string `json:"auth,omitempty"`
	// The username for basic authentication.
	Username *plugins.Secret `json:"username,omitempty"`
	// The password for basic authentication.
	Password *plugins.Secret `json:"password,omitempty"`
}

// Server defines the common parameters for the server plugin
type Server struct {
	CommonFields `json:",inline"`

	// Host defines the IP address or host name of the server.
	Host *string `json:"host,omitempty"`
	// Name defines the name of the server. Used for logging and certificate verification in TLS transport (when the host is the address).
	ServerName *string `json:"name,omitempty"`
	// Port defines the port number of the host. Note that both TCP packets (event stream) and UDP packets (heartbeat messages) are sent to this port.
	Port *string `json:"port,omitempty"`
	// SharedKey defines the shared key per server.
	SharedKey *string `json:"sharedKey,omitempty"`
	// Username defines the username for authentication.
	Username *plugins.Secret `json:"username,omitempty"`
	// Password defines the password for authentication.
	Password *plugins.Secret `json:"password,omitempty"`
	// Standby marks a node as the standby node for an Active-Standby model between Fluentd nodes.
	Standby *string `json:"standby,omitempty"`
	// Weight defines the load balancing weight
	Weight *string `json:"weight,omitempty"`
}

// SDCommon defines the common parameters for the ServiceDiscovery plugin
type SDCommon struct {
	// The @id parameter specifies a unique name for the configuration.
	Id *string `json:"id,omitempty"`
	// The @type parameter specifies the type of the plugin.
	// +kubebuilder:validation:Enum:=static;file;srv
	Type *string `json:"type"`
	// The @log_level parameter specifies the plugin-specific logging level
	LogLevel *string `json:"logLevel,omitempty"`
}

// ServiceDiscovery defines various parameters for the ServiceDiscovery plugin.
// Fluentd has a pluggable system called Service Discovery that lets the user extend and reuse custom output service discovery.
type ServiceDiscovery struct {
	SDCommon `json:",inline,omitempty"`
	// The server section of this plugin
	Server                *Server `json:"server,omitempty"`
	*FileServiceDiscovery `json:",inline,omitempty"`
	*SrvServiceDiscovery  `json:",inline,omitempty"`
}

// FileServiceDiscovery defines the file type for the ServiceDiscovery plugin
type FileServiceDiscovery struct {
	// The path of the target list. Default is '/etc/fluent/sd.yaml'
	Path *string `json:"path,omitempty"`
	// The encoding of the configuration file.
	ConfEncoding *string `json:"confEncoding,omitempty"`
}

// SrvServiceDiscovery defines the srv type for the ServiceDiscovery plugin
type SrvServiceDiscovery struct {
	// Service without the underscore in RFC2782.
	Service *string `json:"service,omitempty"`
	// Proto without the underscore in RFC2782.
	Proto *string `json:"proto,omitempty"`
	// The name in RFC2782.
	Hostname *string `json:"hostname,omitempty"`
	// DnsServerHost defines the hostname of the DNS server to request the SRV record.
	DnsServerHost *string `json:"dnsServerHost,omitempty"`
	// Interval defines the interval of sending requests to DNS server.
	Interval *string `json:"interval,omitempty"`
	// DnsLookup resolves the hostname to IP address of the SRV's Target.
	DnsLookup *string `json:"dnsLookup,omitempty"`
}

func (j *Inject) Name() string {
	return "inject"
}

func (j *Inject) Params(_ plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(j.Name())

	if j.TimeType != nil {
		ps.InsertPairs("time_type", fmt.Sprint(*j.TimeType))
	}
	if j.TimeFormat != nil {
		ps.InsertPairs("time_type", fmt.Sprint(*j.TimeFormat))
	}
	if j.Localtime != nil {
		ps.InsertPairs("localtime", fmt.Sprint(*j.Localtime))
	}
	if j.UTC != nil {
		ps.InsertPairs("utc", fmt.Sprint(*j.UTC))
	}
	if j.Timezone != nil {
		ps.InsertPairs("timezone", fmt.Sprint(*j.Timezone))
	}
	if j.TimeFormatFallbacks != nil {
		ps.InsertPairs("time_format_fallbacks", fmt.Sprint(*j.TimeFormatFallbacks))
	}

	if j.HostnameKey != nil {
		ps.InsertPairs("hostname_key", fmt.Sprint(*j.HostnameKey))
	}
	if j.Hostname != nil {
		ps.InsertPairs("hostname", fmt.Sprint(*j.Hostname))
	}
	if j.WorkerIdKey != nil {
		ps.InsertPairs("worker_id_key", fmt.Sprint(*j.WorkerIdKey))
	}
	if j.TagKey != nil {
		ps.InsertPairs("tag_key", fmt.Sprint(*j.TagKey))
	}
	if j.TimeKey != nil {
		ps.InsertPairs("time_key", fmt.Sprint(*j.TimeKey))
	}
	return ps, nil
}

func (s *Security) Name() string {
	return "security"
}

func (s *Security) Params(loader plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(s.Name())
	if s.SelfHostname != nil {
		ps.InsertPairs("self_hostname", fmt.Sprint(*s.SelfHostname))
	}
	if s.SharedKey != nil {
		ps.InsertPairs("shared_key", fmt.Sprint(*s.SharedKey))
	}
	if s.UserAuth != nil {
		ps.InsertPairs("user_auth", fmt.Sprint(*s.UserAuth))
	}
	if s.AllowAnonymousSource != nil {
		ps.InsertPairs("allow_anonymous_source", fmt.Sprint(*s.AllowAnonymousSource))
	}
	if s.User != nil {
		if s.User.Username != nil && s.User.Password != nil {
			subchild, _ := s.User.Params(loader)
			ps.InsertChilds(subchild)
		}
	}
	return ps, nil
}

func (u *User) Name() string {
	return "user"
}

func (u *User) Params(loader plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(u.Name())
	if u.Username != nil {
		user, err := loader.LoadSecret(*u.Username)
		if err != nil {
			return nil, err
		}
		ps.InsertPairs("username", user)
	}

	if u.Password != nil {
		pwd, err := loader.LoadSecret(*u.Username)
		if err != nil {
			return nil, err
		}
		ps.InsertPairs("password", pwd)
	}

	return ps, nil
}

func (a *Auth) Name() string {
	return "auth"
}

func (a *Auth) Params(loader plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(a.Name())
	if a.Username != nil {
		user, err := loader.LoadSecret(*a.Username)
		if err != nil {
			return nil, err
		}
		ps.InsertPairs("username", user)
	}

	if a.Password != nil {
		pwd, err := loader.LoadSecret(*a.Password)
		if err != nil {
			return nil, err
		}
		ps.InsertPairs("password", pwd)
	}

	if a.Method != nil {
		ps.InsertPairs("method", fmt.Sprint(*a.Method))
	}
	return ps, nil
}

func (t *Transport) Name() string {
	return "transport"
}

func (t *Transport) Params(_ plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore("transport")

	if t.Protocol != nil {
		ps.InsertPairs("protocol", fmt.Sprint(*t.Protocol))
	}
	if t.Version != nil {
		ps.InsertPairs("version", fmt.Sprint(*t.Version))
	}
	if t.Ciphers != nil {
		ps.InsertPairs("ciphers", fmt.Sprint(*t.Ciphers))
	}
	if t.Insecure != nil {
		ps.InsertPairs("insecure", fmt.Sprint(*t.Insecure))
	}
	if t.CaPath != nil {
		ps.InsertPairs("ca_path", fmt.Sprint(*t.CaPath))
	}
	if t.CertPath != nil {
		ps.InsertPairs("cert_path", fmt.Sprint(*t.CertPath))
	}
	if t.PrivateKeyPath != nil {
		ps.InsertPairs("private_key_path", fmt.Sprint(*t.PrivateKeyPath))
	}
	if t.PrivateKeyPassphrase != nil {
		ps.InsertPairs("private_key_passphrase", fmt.Sprint(*t.PrivateKeyPassphrase))
	}
	if t.ClientCertAuth != nil {
		ps.InsertPairs("client_certAuth", fmt.Sprint(*t.ClientCertAuth))
	}
	if t.CaCertPath != nil {
		ps.InsertPairs("ca_certPath", fmt.Sprint(*t.CaCertPath))
	}
	if t.CaPrivateKeyPath != nil {
		ps.InsertPairs("ca_private_key_path", fmt.Sprint(*t.CaPrivateKeyPath))
	}
	if t.CaPrivateKeyPassphrase != nil {
		ps.InsertPairs("ca_private_key_passphrase", fmt.Sprint(*t.CaPrivateKeyPassphrase))
	}
	if t.CertVerifier != nil {
		ps.InsertPairs("cert_verifier", fmt.Sprint(*t.CertVerifier))
	}
	return ps, nil
}

func (c *Client) Name() string {
	return "client"
}

func (c *Client) Params(_ plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore("client")
	if c.Host != nil {
		ps.InsertPairs("host", fmt.Sprint(*c.Host))
	}
	if c.Network != nil {
		ps.InsertPairs("network", fmt.Sprint(*c.Network))
	}
	if c.SharedKey != nil {
		ps.InsertPairs("shared_key", fmt.Sprint(*c.SharedKey))
	}
	if c.Users != nil {
		ps.InsertPairs("users", fmt.Sprint(*c.Users))
	}
	return ps, nil
}

func (s *Server) Name() string {
	return "server"
}

func (s *Server) Params(loader plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(s.Name())

	if s.Id != nil {
		ps.InsertPairs("@id", fmt.Sprint(*s.Id))
	}
	if s.Type != nil {
		ps.InsertType(fmt.Sprint(*s.Type))
	}
	if s.LogLevel != nil {
		ps.InsertPairs("@log_level", fmt.Sprint(*s.LogLevel))
	}

	if s.Host != nil {
		ps.InsertPairs("host", fmt.Sprint(*s.Host))
	}
	if s.ServerName != nil {
		ps.InsertPairs("server", fmt.Sprint(*s.ServerName))
	}
	if s.Port != nil {
		ps.InsertPairs("port", fmt.Sprint(*s.Port))
	}
	if s.SharedKey != nil {
		ps.InsertPairs("shared_key", fmt.Sprint(*s.SharedKey))
	}
	if s.Username != nil {
		user, err := loader.LoadSecret(*s.Username)
		if err != nil {
			return nil, err
		}

		ps.InsertPairs("username", user)
	}
	if s.Password != nil {
		pwd, err := loader.LoadSecret(*s.Password)
		if err != nil {
			return nil, err
		}

		ps.InsertPairs("password", pwd)
	}
	if s.Standby != nil {
		ps.InsertPairs("standby", fmt.Sprint(*s.Host))
	}
	if s.Weight != nil {
		ps.InsertPairs("weight", fmt.Sprint(*s.Host))
	}

	return ps, nil
}

func (sd *ServiceDiscovery) Name() string {
	return "service_discovery"
}

func (sd *ServiceDiscovery) Params(loader plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(sd.Name())
	childs := make([]*params.PluginStore, 0)
	if sd.Id != nil {
		ps.InsertPairs("@id", fmt.Sprint(*sd.Id))
	}

	ps.InsertType(fmt.Sprint(*sd.Type))

	if sd.LogLevel != nil {
		ps.InsertPairs("@log_level", fmt.Sprint(*sd.LogLevel))
	}

	if sd.Server != nil {
		child, _ := sd.Server.Params(loader)
		childs = append(childs, child)
	}

	if sd.FileServiceDiscovery != nil {
		if sd.FileServiceDiscovery.Path != nil {
			ps.InsertPairs("path", fmt.Sprint(*sd.FileServiceDiscovery.Path))
		}
		if sd.FileServiceDiscovery.ConfEncoding != nil {
			ps.InsertPairs("conf_encoding", fmt.Sprint(*sd.FileServiceDiscovery.ConfEncoding))
		}
	}

	if sd.SrvServiceDiscovery != nil {
		if sd.SrvServiceDiscovery.Service != nil {
			ps.InsertPairs("service", fmt.Sprint(*sd.SrvServiceDiscovery.Service))
		}
		if sd.SrvServiceDiscovery.Proto != nil {
			ps.InsertPairs("proto", fmt.Sprint(*sd.SrvServiceDiscovery.Proto))
		}
		if sd.SrvServiceDiscovery.Hostname != nil {
			ps.InsertPairs("hostname", fmt.Sprint(*sd.SrvServiceDiscovery.Hostname))
		}
		if sd.SrvServiceDiscovery.DnsServerHost != nil {
			ps.InsertPairs("dns_server_host", fmt.Sprint(*sd.SrvServiceDiscovery.DnsServerHost))
		}
		if sd.SrvServiceDiscovery.Interval != nil {
			ps.InsertPairs("interval", fmt.Sprint(*sd.SrvServiceDiscovery.Interval))
		}
		if sd.SrvServiceDiscovery.DnsLookup != nil {
			ps.InsertPairs("dns_lookup", fmt.Sprint(*sd.SrvServiceDiscovery.DnsLookup))
		}
	}
	ps.InsertChilds(childs...)
	return ps, nil
}
