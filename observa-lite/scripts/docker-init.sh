#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
umask 077
if [ -e .env ]; then
  echo 'O arquivo .env já existe. Nenhum token foi alterado.'
  exit 0
fi
random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  elif [ -r /dev/urandom ]; then
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
  else
    echo 'Sem gerador de segredos seguro (/dev/urandom).' >&2
    exit 1
  fi
}
admin="$(random_secret)"
agent="$(random_secret)"
if [ "$admin" = "$agent" ] || [ "${#admin}" -lt 64 ] || [ "${#agent}" -lt 64 ]; then
  echo 'Falha na geração de credenciais.' >&2
  exit 1
fi
cat > .env <<EOT
OBSERVA_ADMIN_TOKEN=$admin
OBSERVA_AGENT_TOKEN=$agent
OBSERVA_BIND_ADDR=127.0.0.1
OBSERVA_PORT=8080
OBSERVA_AGENT_INTERVAL=30s
OBSERVA_ALLOW_PRIVATE_TARGETS=false
OBSERVA_LOKI_PORT=3100
EOT
chmod 600 .env
echo 'Credenciais geradas no .env com permissão 600 (não exibidas).'
echo "Para ler seu token de acesso: sed -n 's/^OBSERVA_ADMIN_TOKEN=//p' .env"
