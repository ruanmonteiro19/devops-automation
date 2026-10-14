#!/bin/sh
# Cross-distribution Linux installer: systemd when available, foreground otherwise.
# Run from extracted source/bundle. Does not open a firewall or expose HTTP publicly.
set -eu
MODE=${1:-all}
case "$MODE" in server|agent|all) ;; *) echo "Uso: sudo ./scripts/install.sh [all|server|agent]" >&2; exit 2;; esac
if [ "$(id -u)" != 0 ]; then echo "Execute como root (sudo)." >&2; exit 1;fi
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
ARCH=$(uname -m)
case "$ARCH" in x86_64|amd64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) echo "Arquitetura $ARCH: instale Go 1.23+ e compile manualmente." >&2;exit 1;; esac
if [ -f "$ROOT/dist/observa-linux-$ARCH" ];then
 install -m 755 "$ROOT/dist/observa-linux-$ARCH" /usr/local/bin/observa
elif command -v go >/dev/null 2>&1;then
 (cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -trimpath -ldflags='-s -w' -o /usr/local/bin/observa ./cmd/observa)
else
 echo "Binário ausente. Instale Go 1.23+ ou use um pacote de release com dist/observa-linux-$ARCH." >&2;exit 1
fi
mkdir -p /etc/observa /var/lib/observa
chmod 700 /etc/observa /var/lib/observa
randhex(){od -An -N32 -tx1 /dev/urandom | tr -d ' \n';}
if [ "$MODE" = all ] || [ "$MODE" = server ];then
 ADMIN_TOKEN=${OBSERVA_ADMIN_TOKEN:-$(randhex)}
 AGENT_TOKEN=${OBSERVA_AGENT_TOKEN:-$(randhex)}
 if [ -f /etc/observa/server.env ];then echo "Configuração /etc/observa/server.env já existe; preservando-a.";else
  printf 'OBSERVA_ADMIN_TOKEN=%s\nOBSERVA_AGENT_TOKEN=%s\nOBSERVA_LISTEN=127.0.0.1:8080\nOBSERVA_DB=/var/lib/observa/state.json\nOBSERVA_ALLOW_PRIVATE_TARGETS=false\n' "$ADMIN_TOKEN" "$AGENT_TOKEN" > /etc/observa/server.env
  chmod 600 /etc/observa/server.env
  echo "CHAVE ADMINISTRATIVA (guarde em local seguro): $ADMIN_TOKEN"
 fi
 if command -v useradd >/dev/null 2>&1 && ! id observa >/dev/null 2>&1;then useradd --system --home-dir /var/lib/observa --shell /usr/sbin/nologin observa 2>/dev/null || true;fi
 if id observa >/dev/null 2>&1;then chown observa:observa /var/lib/observa;fi
 if [ -d /run/systemd/system ];then
  cat >/etc/systemd/system/observa-server.service <<'UNIT'
[Unit]
Description=Observa Lite API and Web
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
User=observa
Group=observa
EnvironmentFile=/etc/observa/server.env
ExecStart=/usr/local/bin/observa server
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/observa
PrivateTmp=true
[Install]
WantedBy=multi-user.target
UNIT
  systemctl daemon-reload
  systemctl enable --now observa-server
 else echo "Sem systemd. Inicie manualmente: . /etc/observa/server.env; export OBSERVA_ADMIN_TOKEN OBSERVA_AGENT_TOKEN OBSERVA_DB; observa server";fi
fi
if [ "$MODE" = all ] || [ "$MODE" = agent ];then
 if [ "$MODE" = all ];then
  # Reuse server secret when installing on a single host; do not print it.
  . /etc/observa/server.env
  SERVER_URL=http://127.0.0.1:8080
 else
  if [ -z "${OBSERVA_AGENT_TOKEN:-}" ];then echo "Defina OBSERVA_AGENT_TOKEN ao instalar o agente remoto." >&2;exit 2;fi
  SERVER_URL=${OBSERVA_SERVER:-}
  case "$SERVER_URL" in https://*) ;; *) echo "Agente remoto exige OBSERVA_SERVER=https://..." >&2;exit 2;;esac
 fi
 if [ -f /etc/observa/agent.env ];then echo "Configuração /etc/observa/agent.env já existe; preservando-a.";else
  printf 'OBSERVA_AGENT_TOKEN=%s\nOBSERVA_SERVER=%s\n' "$OBSERVA_AGENT_TOKEN" "$SERVER_URL" >/etc/observa/agent.env
  chmod 600 /etc/observa/agent.env
 fi
 if [ -d /run/systemd/system ];then
  cat >/etc/systemd/system/observa-agent.service <<'UNIT'
[Unit]
Description=Observa Lite passive Linux discovery agent
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
EnvironmentFile=/etc/observa/agent.env
ExecStart=/usr/local/bin/observa agent
Restart=always
RestartSec=10
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
PrivateTmp=true
[Install]
WantedBy=multi-user.target
UNIT
  systemctl daemon-reload
  systemctl enable --now observa-agent
 else echo "Sem systemd. Inicie manualmente: . /etc/observa/agent.env; export OBSERVA_AGENT_TOKEN OBSERVA_SERVER; observa agent";fi
fi
echo "Instalação concluída. A interface local: http://127.0.0.1:8080"
