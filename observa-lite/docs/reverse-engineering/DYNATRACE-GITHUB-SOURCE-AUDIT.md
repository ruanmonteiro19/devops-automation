# Engenharia reversa baseada no código público do GitHub — Dynatrace → Observa Lite
Data: 2026-10-14. Escopo: verificação de implementações públicas (não acesso ao código proprietário do OneAgent, ActiveGate ou backend Dynatrace).

## Método e limites
- Fontes oficiais inspecionadas: `Dynatrace/dynatrace-operator` (Apache-2.0, trechos com SPDX), `Dynatrace/OneAgent-SDK` (interfaces Java), além de `coroot/coroot` e `grafana/grafana` como referências complementares.
- A engenharia reversa aqui é **comportamental e arquitetural**, com exame estático de código público e mapeamento testável. NÃO decompilamos nem extraímos binários ou software proprietário.
- Repositório-alvo: `ruanmonteiro19/devops-automation`, branch `feature/agents-gateway-i18n`.
- Versão atual do Observa Lite **não** tem descoberta dinâmica de Gateway, failover de Network Zone, OTLP APM nem Smartscape completo. Não confundir o nome das telas com as funções prontas.

## 1. ActiveGate: capacidades e funções são modulares
**Fonte:** [Dynatrace Operator, spec.go](https://github.com/Dynatrace/dynatrace-operator/blob/main/pkg/api/latest/dynakube/activegate/spec.go)

Na declaração `Capability` aparecem as capacidades: `routing` → `MSGrouter`, `kubernetes-monitoring` → `kubernetes_monitoring`, `metrics-ingest` → `metrics_ingest`, `dynatrace-api` → `restInterface`, `debugging` → `debugging`. O mesmo arquivo prevê certificado TLS por secret, réplicas, recursos, variáveis de ambiente, configuração, StatefulSet e tolerâncias. Isso prova que o operador **provisiona recursos e capacidades separáveis**, não que todos os mecanismos internos do ActiveGate sejam públicos.

**Aplicação no Observa Lite:** o `cmd/observa-gateway/main.go` atual é um relay HTTPS restrito a `POST /api/v1/ingest`, com token e mTLS opcional. NÃO substitui ActiveGate real. Evoluir com capacidades declarativas independentes `ingest-routing`, `metrics-ingest`, `k8s-api-monitoring`, `extension-execution` apenas quando implementadas, cada uma com RBAC, parâmetros, métricas e healthchecks.

**Teste de aceite:** iniciar relay com só routing, validar que APIs administrativas e outros caminhos respondem 404; desligar um módulo sem afetar os demais; testar TLS por CA e identidade SAN.

## 2. Network Zone não é apenas uma string de etiqueta
**Fonte:** [oneagent/connection_info.go](https://github.com/Dynatrace/dynatrace-operator/blob/main/pkg/clients/dynatrace/oneagent/connection_info.go)

O método `GetConnectionInfo` realiza GET em `/v1/deployment/installer/agent/connectioninfo`. Quando configurada, envia `networkZone` e `defaultZoneFallback=true`; recebe `tenantUUID`, `tenantToken` e lista `communicationEndpoints`. A lista é normalizada por `uniqueEndpoints()`, **preservando ordem e removendo duplicações**. Em modo restrito, `missingRequiredHosts()` verifica se aparece o host do serviço ActiveGate local esperado. O código define estados transitórios `NoCommunicationEndpointsError` e `StaleNetworkZoneEndpointsError`.

**Fonte:** [oneagent/connectioninfo/reconciler.go](https://github.com/Dynatrace/dynatrace-operator/blob/main/pkg/controllers/dynakube/connectioninfo/oneagent/reconciler.go)

O reconciliador suspende a progressão quando a relação entre zona e ActiveGate local está desatualizada, atualiza o status e guarda credenciais de forma controlada em Kubernetes Secrets. O código trata erros de disponibilidade temporária de endpoints de maneira diferente de erros permanentes.

**Aplicação no Observa Lite:** em vez do `OBSERVA_SERVER` único, definir `GET /api/v1/agent/connectioninfo`, protegido por autenticação de agente (idealmente credenciais por agente, não token global):
- parâmetros: `network_zone`, `host_id`, versão do protocolo;
- resposta: versão e validade da configuração, lista ordenada e não vazia de URLs HTTPS de gateway, fallback explícito e política por zona;
- o agent valida FQDN/SAN/CA, tenta destinos por ordem e registra seleção e causa de erro;
- jamais aceitar URLs de gateway encaminhadas sem assinatura/autenticação confiável, para evitar redirecionamento de tokens;
- healthcheck/heartbeat de gateways com timestamps, TTL e mecanismo de quarentena; tratar vazio/stale como estados explícitos, não inventar disponibilidade;
- usar descoberta fora da rota de ingestão para evitar loop com gateway indisponível;
- fila local de pequena capacidade, TTL, limites de memória/disco, descarte controlado, telemetria própria e backoff com jitter.

**Estado hoje:** apenas `host_group` e `network_zone` enviados/armazenados. Failover/discovery por zona NÃO existe.

**Aceite:** instalar dois gateways na zona A + um na zona default; Agent A escolhe A e não default enquanto A estiver disponível; matar gateway escolhido e verificar fallback controlado e nenhuma duplicação significativa de ingestão; configurar zona estrita e verificar rejeição quando não houver local; bloquear CA não confiável.

## 3. Host Group: agrupamento distinto da conectividade
**Fontes:** [dynakube.go](https://github.com/Dynatrace/dynatrace-operator/blob/main/pkg/clients/dynatrace/dynakube.go), [oneagent/props.go](https://github.com/Dynatrace/dynatrace-operator/blob/main/pkg/api/latest/dynakube/oneagent/props.go)

O Operator passa `WithNetworkZone()` e `WithHostGroup()` como opções independentes. A propriedade HostGroup também pode ser derivada de `--set-host-group`. Isto comprova que o agrupamento de hosts e o roteamento não são a mesma configuração.

**Aplicação:** separar `HostGroup` (organização, filtro, política de descoberta) de `NetworkZone` (topologia de comunicação, preferência de gateways, fallback). Não deduzir um a partir do outro. Suporte hierárquico e regras por grupo exigem armazenamento e APIs próprios, com testes de migração.

## 4. Deep Monitoring: modos e injeção são recursos distintos
**Fontes:** [oneagent/props.go](https://github.com/Dynatrace/dynatrace-operator/blob/main/pkg/api/latest/dynakube/oneagent/props.go), [webhook/mutator/oneagent/env.go](https://github.com/Dynatrace/dynatrace-operator/blob/main/pkg/webhook/mutation/pod/mutator/oneagent/env.go), [ARCHITECTURE.md](https://github.com/Dynatrace/dynatrace-operator/blob/main/ARCHITECTURE.md)

`IsApplicationMonitoringMode`, `IsCloudNativeFullstackMode`, `IsHostMonitoringMode`, `IsClassicFullStackMode` estão separados. `IsAppInjectionNeeded()` e `IsDaemonsetRequired()` expressam decisões diferentes. O mutator acrescenta metadados, Network Zone e, quando apropriado, um `LD_PRELOAD`, preservando valor já existente. São mecanismos de implantação observáveis; não disponibilizam o código proprietário dos sensores.

**Aplicação:** política opt-in de instrumentação específica por tecnologia, workload e namespace. Suportar primeiro OpenTelemetry Java Agent e .NET auto-instrumentation por meios documentados, com rollout e rollback. Fazer ingestão OTLP, persistir spans ou agregações com retenção explícita, propagar W3C Trace Context e derivar service map REAL. NÃO alterar processos de produção sem aprovação.

**Aceite:** dois serviços instrumentados `api` e `db-client` comunicam com propagação traceparent; são vistos como serviços distintos com latência, erro e relações fundamentadas em spans, mesmo se seus processos são executados no mesmo host.

## 5. Serviços APM exigem dados L7, não somente PID
**Fonte oficial pública:** [OneAgent-SDK, OneAgentSDK.java](https://github.com/Dynatrace/OneAgent-SDK/blob/master/api/com/dynatrace/oneagent/sdk/api/OneAgentSDK.java), [IncomingWebRequestTracer.java](https://github.com/Dynatrace/OneAgent-SDK/blob/master/api/com/dynatrace/oneagent/sdk/api/IncomingWebRequestTracer.java), [OutgoingWebRequestTracer.java](https://github.com/Dynatrace/OneAgent-SDK/blob/master/api/com/dynatrace/oneagent/sdk/api/OutgoingWebRequestTracer.java), [DatabaseRequestTracer.java](https://github.com/Dynatrace/OneAgent-SDK/blob/master/api/com/dynatrace/oneagent/sdk/api/DatabaseRequestTracer.java)

As interfaces especificam request de entrada/saída, método, URL, status HTTP, headers, propagação de contexto W3C, chamadas SQL e atributos adicionais. Em particular, `createWebApplicationInfo(webServerName,applicationID,contextRoot)` demonstra que nome/identidade de serviço usam dados lógicos, não só executável. Isso é uma **API pública**, não a implementação interna do OneAgent.

**Estado Observa Lite:** passamos a separar `process_map` do `service_map`, mas atualmente o segundo fica vazio porque ainda não há ingestão real de spans. É a conduta correta até instrumentação efetiva existir.

**Aceite:** injetar OTLP real no servidor, identificar serviço por `service.name`, `service.namespace`, `service.instance.id`, timestamps, span.kind, server/client, status, atributos HTTP/db e relacionamento trace.parent_id. Rejeitar a promoção de sockets/nomes de processo a entidades APM confirmadas.

## 6. Topologia: modelo aberto de agregação para implementação própria
**Fontes complementares NÃO Dynatrace:** [Coroot service_map.go](https://github.com/coroot/coroot/blob/main/api/views/overview/service_map.go), [Coroot model/connection.go](https://github.com/coroot/coroot/blob/main/model/connection.go), [Coroot constructor/connections.go](https://github.com/coroot/coroot/blob/main/constructor/connections.go).

O código aberto do Coroot agrega relações de aplicações/instâncias e calcula status e métricas de conexão. Usar como estudo arquitetural, respeitando licenças. Smartscape proprietary não está disponibilizado como código completo no repositório da Dynatrace. Para reproduzir experiência similar, construir nosso próprio modelo de grafo L7 com causalidade/evidência; comparar comportamento com APIs oficiais/ambiente autorizado.

## 7. Painéis: importar Grafana é serviço com validação
**Fontes complementares NÃO Dynatrace:** [Grafana dashboardimport/service.go](https://github.com/grafana/grafana/blob/main/pkg/services/dashboardimport/service/service.go), [Grafana dashboardimport/api.go](https://github.com/grafana/grafana/blob/main/pkg/services/dashboardimport/api/api.go).

O Grafana tem pipeline dedicado de importação/transformação/validação de dashboards. Não assumir que guardar JSON de painéis no banco equivalha a importação Grafana. Validar JSON Classic e resource schemas, reconhecer datasources, variáveis, grid positions e avisar quais plugins/painéis não são suportados. Atenção à AGPL/licenças.

## 8. Gap audit dos arquivos atuais
| Arquivo do Observa Lite | Diagnóstico a partir da comparação |
| --- | --- |
| `cmd/observa/main.go` | Agent com destino único `OBSERVA_SERVER`, TLS/CA/mTLS opcionais; ainda sem obtenção periódica de endpoints por zona, failover com política e fila offline |
| `cmd/observa-gateway/main.go` | Gateway HTTPS com relay autenticado; sem registro, heartbeat, store-and-forward, multiprotocolo ou capabilities modulares |
| `internal/model/model.go` | `HostGroup` e `NetworkZone` são metadados; modelo ainda sem Gateway, status, connection policy, spans, trace relationships ou entidades APM |
| `internal/server/server.go` | REST e ingestão de discovery, sem control-plane de connectioninfo nem OTLP receiver |
| `internal/store/store.go` | Process Mapping separado do Service Mapping, sem grafo APM proveniente de L7 instrumentado |
| `frontend/src/components/observa-app.tsx` | UI de Gateway/Grupos/Zonas existe mas não é console de configuração operacional de gateways reais |

## 9. Ordem de implementação com provas
1. **Control-plane de Gateways por zona**: registro/heartbeat, API connectioninfo, endpoint routing, TLS, failover limitado, testes de 2 gateways e zona strict/default.
2. **Ingestão OTLP de spans**: receptor autêntico, modelo de serviços, processador de trace context, RED metrics, Service Mapping proveniente de dados L7.
3. **Deploy/monitoring modes**: instrumentação Java/.NET por opt-in, Linux infra/process discovery separados, integrações OpenShift.
4. **Smartscape UI**: camadas, evidência do grafo, saúde e drilldown, desempenho e UX.
5. **Grafana dashboards/import**: parser/validador e mapeamento; testes de dashboards reais, sem incompatibilidades silenciosas.
6. **Security/enterprise**: TLS permanente, SSO/OIDC, MFA por IdP, RBAC, gestão de segredos, logs/auditoria, versões/rollbacks e quotas.

## 10. Critérios de honestidade
- **Código público oficial**: evidência direta e citada no texto.
- **Comportamento inferido**: hipótese explícita até validação black-box em instalação autorizada.
- **Proposta Observa Lite**: arquitetura nova, não declaração de funcionalidade do Dynatrace.
- **Implementado**: só após commit.
- **Testado**: só após CI e testes de runtime separados.
- **Homologado**: apenas após teste em ambiente de destino e critérios de performance/segurança.

**Regra:** não modificar a instalação `x-oc-grafana` ou serviços existentes sem aprovação explícita. O relatório documenta fontes públicas; não implica acesso ao código proprietário Dynatrace.
