#/bin/bash

trap 'kill %1; wait' SIGTERM
iptables -A INPUT -p tcp --dport {{ .KubeRBACProxyPort }} -j ACCEPT -m comment --comment "valitail"
/usr/bin/telegraf --config /etc/telegraf/telegraf.conf &
wait
