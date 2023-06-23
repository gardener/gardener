[[outputs.prometheus_client]]
## Address to listen on.
listen = ":{{ .ListenPort }}"
metric_version = 2
# Gather packets and bytes throughput from iptables
[[inputs.iptables]]
## iptables require root access on most systems.
## Setting 'use_sudo' to true will make use of sudo to run iptables.
## Users must configure sudo to allow telegraf user to run iptables with no password.
## iptables can be restricted to only list command "iptables -nvL".
use_sudo = true
## defines the table to monitor:
table = "filter"
## defines the chains to monitor.
## NOTE: iptables rules without a comment will not be monitored.
## Read the plugin documentation for more information.
chains = [ "INPUT" ]
