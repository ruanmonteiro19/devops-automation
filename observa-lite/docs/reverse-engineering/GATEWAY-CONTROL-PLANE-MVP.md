# Observa Lite — Control-plane de Gateways por Network Zone

## Estado da implementação
Este documento é estritamente para a versão em `feature/agents-gateway-i18n`. Configuração de rotas no servidor e **failover por tentativas** foram implementados como funcionalidade opcional. O ambiente atual `x-oc-grafana` NÃO foi reconfigurado por este commit.

### O que já existe
- `GET /api/v1/gateways` — lista de rotas, autenticado como administrador.
- `POST /api/v1/gateways` — cadastrar/substituir destino HTTPS autenticado como administrador.
- `DELETE /api/v1/gateways/{id}` — remover rota autenticado como administrador.
- `GET /api/v1/agent/connectioninfo?network_zone=zone-a&allow_default_fallback=true` — somente com chave do Agent. Fornece URLs aprovadas ordenadas por prioridade (menor número primeiro), elimina duplicatas, faz fallback explícito da zona `default` quando solicitado.
- Agent opt-in `--control-plane URL`, `--network-zone ZONE`, `--allow-default-zone-fallback`. Busca a lista autenticada e tenta o próximo Gateway quando o primeiro apresenta falha de transporte, HTTP 5xx ou 429. Erros HTTP 4xx (exceto 429) interrompem o envio.
- URLs cadastradas exigem esquema `https://`, sem credenciais embutidas, caminho/query/fragment. O Agent só aceita lista obtida de origem HTTPS verificada (ou HTTP loopback para laboratório).
- Frontend `/gateways`: formulário administrativo para listar, cadastrar e excluir destinos. Não mostra `online` enquanto não houver heartbeat.
- A persistência JSON existente é mantida; um novo campo `gateways` é adicionado sem apagar hosts, templates, datasources ou dashboards.

### O que isso NÃO representa
- **Não há descoberta automática de gateways** e nem heartbeat. As URLs são configuradas por um administrador autorizado; um registro não implica que o Gateway esteja vivo.
- **Não há garantia de exactly-once**: após timeout do HTTP 202, não é possível assegurar se o primeiro Gateway já entregou o evento; tentativas em outro destino podem gerar duplicatas. Futuramente: `batch_id`, deduplicação e confirmação fim a fim.
- **Não há fila offline durável** no Agent. Se o control-plane falhar, o Agent para de enviar até a próxima coleta.
- Não existe mTLS de identidade individual/rotação automática de credenciais por Agent: o sistema de laboratório usa chave de agente compartilhada.
- `Host Group` organiza inventário; não seleciona Gateway. `Network Zone` filtra/preferencializa rotas.
- Não existe APM Deep Monitoring real (Java/.NET), nem telemetria L7 via OTLP.

### Exemplo de fluxo
1. Operador implanta duas instâncias de Observa Gateway com certificados válidos para suas respectivas URLs, autenticação, upstream protegido e acesso de rede autorizado. **Não executar em servidores de produção sem homologação.**
2. Operador acessa a interface `Gateways` via HTTPS e cadastra `gateway-a.example` com `network_zone=zone-a`, `priority=10`.
3. Opcionalmente cadastra `gateway-b.example` em `zone-a` com `priority=20`, e um fallback em `default` com `priority=50`.
4. No Agent de laboratório, após validar TLS, definir `OBSERVA_CONTROL_PLANE=https://observa.example`, `OBSERVA_NETWORK_ZONE=zone-a`, e `OBSERVA_DEFAULT_ZONE_FALLBACK=true` somente se o fallback for permitido.
5. O Agent consulta a lista com sua chave, valida certificados e tenta as URLs na ordem retornada. Uma falha 503 no primeiro destino faz tentar o segundo. Erro 401 interrompe.

### Segurança
- Proteja a interface e as APIs administrativas com **HTTPS e firewall/ACL**. O acesso via `http://10.3.1.207:18080` atual não deve ser usado para cadastrar Gateways ou transportar tokens pela rede.
- Use CA corporativa ou certificado SAN correspondente ao FQDN usado pelo Agent. Não use `insecureSkipVerify`.
- Como `OBSERVA_AGENT_TOKEN` é compartilhado, trate-o como segredo de alto impacto. Não cole tokens em logs/chats.
- Apenas URL configurada por administrador pode entrar no retorno de `connectioninfo`. Evitar controle dessas rotas por agentes não confiáveis.
- Esta versão não faz validação ativa do certificado no ato do cadastro; faz validação na conexão de cada Agent.

### Testes automáticos relevantes
- `internal/store/gateway_test.go`: prioridade, isolamento por zona, fallback opcional, persistência e recusa URLs inseguras.
- `internal/server/controlplane_test.go`: registro/admin, isolamento de autorização e consulta autenticada.
- `internal/routing/routing_test.go`: descoberta autenticada e failover após erro 503 de um dos relays HTTPS, sem desabilitar verificação de certificados.
- `frontend/scripts/qa-visual.cjs`: cadastro, leitura e exclusão pela interface Chromium num ambiente descartável.
- Windows Agent e Linux Agent compilados em CI. O modo opcional de roteamento não deve ser habilitado no ambiente real até testes de certificados e rede.

## Confronto com o código oficial público do Dynatrace
Os conceitos de `networkZone`, `defaultZoneFallback`, `communicationEndpoints`, deduplicação e exigência do ActiveGate local vêm de [oneagent/connection_info.go](https://github.com/Dynatrace/dynatrace-operator/blob/main/pkg/clients/dynatrace/oneagent/connection_info.go). A lógica do Observa Lite é uma implementação própria, funcionalmente menor, e não contém componentes proprietários do OneAgent.
