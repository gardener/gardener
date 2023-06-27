[Unit]
Description=Gardener Node Agent {{ .Version }}
After=network.target

[Service]
LimitMEMLOCK=infinity
Environment=CONFIG=/etc/gardener/node-agent.config
ExecStart=/usr/local/bin/gardener-node-agent
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
