#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ -e deploy/snmp/snmp.yml ]; then
  echo 'Configuração SNMP existente preservada.'
  exit 0
fi
if [ "$(id -u)" != 0 ]; then
  echo 'Execute como root para preparar o arquivo para o UID 65534 do exporter.' >&2
  exit 1
fi
mkdir -p deploy/snmp
umask 077
tmp="$(mktemp deploy/snmp/snmp.yml.XXXXXX)"
trap 'rm -f "$tmp"' EXIT HUP INT TERM
curl --fail --location --proto '=https' --tlsv1.2 \
  https://raw.githubusercontent.com/prometheus/snmp_exporter/v0.30.1/snmp.yml \
  -o "$tmp"
if [ ! -s "$tmp" ]; then echo 'Configuração upstream vazia.' >&2; exit 1; fi
chown 65534:65534 "$tmp"
chmod 600 "$tmp"
mv "$tmp" deploy/snmp/snmp.yml
echo 'Configuração upstream v0.30.1 preparada. Ajuste auth/community/SNMPv3 antes da coleta.'
echo 'Depois: docker compose --profile network up -d snmp-exporter'
