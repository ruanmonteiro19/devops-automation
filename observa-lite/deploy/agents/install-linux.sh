#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo 'Execute com sudo' >&2; exit 1; }
[ "$#" -eq 5 ] || { echo 'Uso: OBSERVA_AGENT_TOKEN=... sh install-linux.sh binario url grupo zona ca.pem-ou--' >&2; exit 2; }
bin=$1
url=$2
group=$3
zone=$4
ca=$5
case "$url" in https://*|http://127.0.0.1:*|http://localhost:*) ;; *) echo 'Exige HTTPS remoto' >&2; exit 2;; esac
[ -n "$OBSERVA_AGENT_TOKEN" ] || { echo 'Token ausente' >&2; exit 2; }
install -d -m 0700 /etc/observa-agent
install -m 0755 "$bin" /usr/local/bin/observa-agent
if [ "$ca" != "-" ]; then install -m 0600 "$ca" /etc/observa-agent/ca.pem; ca=/etc/observa-agent/ca.pem; else ca=""; fi
umask 077
{
 printf 'OBSERVA_AGENT_TOKEN=%s\n' "$OBSERVA_AGENT_TOKEN"
 printf 'OBSERVA_SERVER=%s\n' "$url"
 printf 'OBSERVA_HOST_GROUP=%s\n' "$group"
 printf 'OBSERVA_NETWORK_ZONE=%s\n' "$zone"
 [ -z "$ca" ] || printf 'OBSERVA_CA_FILE=%s\n' "$ca"
} >/etc/observa-agent/agent.env
chmod 600 /etc/observa-agent/agent.env
cat >/etc/systemd/system/observa-agent.service <<'UNIT'
[Unit]
Description=Observa Lite Agent
Wants=network-online.target
After=network-online.target
[Service]
Type=simple
EnvironmentFile=/etc/observa-agent/agent.env
ExecStart=/usr/local/bin/observa-agent agent
Restart=always
RestartSec=10
NoNewPrivileges=true
ProtectSystem=strict
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable observa-agent
systemctl restart observa-agent
echo 'Instalado. Verifique: systemctl status observa-agent'
