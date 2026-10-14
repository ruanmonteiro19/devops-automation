# Observa Lite: agent Linux, Windows e Gateway TLS

Este pacote é um MVP de descoberta por sistema operacional, não faz APM Deep Monitoring automático.

Linux: binários observa-linux-amd64 e observa-linux-arm64, lê /proc.
Windows: binários observa-windows-amd64.exe e observa-windows-arm64.exe, consulta processos e TCP com PowerShell 5.1/CIM.
Gateway: observa-gateway-linux-amd64/arm64 usa TLS obrigatório para ingresso externo, pode exigir certificado de cliente --client-ca.

Host Groups e Network Zones são registrados no inventário e enviados pelo agente. O roteamento prioritário/failover de gateways por zona ainda é uma etapa futura.
Modos reais: discovery. Modos infra/deep ainda devem ser implementados. SSO/MFA também não estão concluídos.

Executar agente em terminal de laboratório:
OBSERVA_AGENT_TOKEN='<TOKEN>' OBSERVA_SERVER='https://gateway.intranet:18443' OBSERVA_HOST_GROUP='grupo.exemplo' OBSERVA_NETWORK_ZONE='zona.interna' OBSERVA_CA_FILE='./ca.pem' ./observa-linux-amd64 agent

Windows PowerShell:
$env:OBSERVA_AGENT_TOKEN='<TOKEN>'; $env:OBSERVA_SERVER='https://gateway.intranet:18443'; $env:OBSERVA_HOST_GROUP='grupo.exemplo'; $env:OBSERVA_NETWORK_ZONE='zona.interna'; $env:OBSERVA_CA_FILE='C:\certs\gateway-ca.pem'; .\observa-windows-amd64.exe agent

Nunca enviar tokens ao chat, não desabilitar validação TLS e não instalar em produção sem autorização.
Para instalar como serviço/tarefa, auditar antes os scripts incluídos no pacote.
Tema preto/verde, seletor de idiomas PT/EN/ES com tradução parcial inicial.
Skills utilizadas: Vercel React Best Practices / Verification. Skills.sh de terceiros só após auditoria de scripts e permissões.

Compatibilidade: servidor da branch antiga ainda não aceita host_group, network_zone ou monitoring_mode. Atualize PRIMEIRO o backend num teste controlado. Enquanto isso, para coletar somente inventário no backend antigo, defina OBSERVA_LEGACY_PAYLOAD=true no agente; grupos e zonas NÃO serão transmitidos nesse modo.
