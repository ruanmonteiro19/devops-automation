# OBSERVA LITE ENTERPRISE — AUDITORIA DE ESCOPO E GAPS FUNCIONAIS

**Data:** 14/10/2026. **Fonte:** revisão estática da branch `feature/agents-gateway-i18n` em `48f306a90bf03f565c71cd01db7345c6bb6bc2c4`. **Escopo:** desiderata do projeto Dynatrace + Grafana + Zabbix versus código implementado.

## Regra de classificação

- **BÁSICO:** funcionalidade reduzida com alguma implementação real; NÃO representa paridade com produtos de referência nem homologação em produção.
- **PARCIAL:** partes de backend/UI existem, mas o comportamento fim a fim ou algumas garantias importantes faltam.
- **SÓ INTERFACE:** menu/página/placeholder existe, sem função operacional correspondente.
- **AUSENTE:** nenhum suporte implementado demonstrável no código inspecionado.
- **VALIDAÇÃO INCOMPLETA:** suporte potencial ou componentes existem, mas faltam provas.
- Um processo executável, ícone, menu, tabela ou endpoint vazio NÃO significa funcionalidade concluída. O status se refere ao código inspecionado, não à instalação real em cada VM.
- A branch de laboratório não deve ser promovida a produção sem testes de segurança, performance, persistência e instalação.
- **IMPORTANTE:** os números abaixo representam itens de escopo analisados, não uma porcentagem de engenharia concluída. Vários itens variam enormemente em complexidade.

## Resultado da auditoria

**Itens de escopo:** 147. **Distribuição:** BÁSICO: 21 | AUSENTE: 95 | PARCIAL: 22 | SÓ INTERFACE: 8 | VALIDAÇÃO INCOMPLETA: 1.

O MVP oferece discovery básico, verificação HTTP/TCP, acesso sob demanda a Prometheus/JSON/Loki, gateway TLS, registro de endpoints por Network Zone, processos/IP topology, dashboard elementar, catálogo estático de extensões e MCP parcial. **Não entrega paridade com Dynatrace nem Grafana nem Zabbix.**

### Evidências no código verificado

- [Modelo de dados](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/internal/model/model.go)
- [Coleta e regras](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/internal/collectors/collectors.go)
- [Roteador Agent](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/internal/routing/routing.go)
- [Gateway](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/cmd/observa-gateway/main.go)
- [Backend/API](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/internal/server/server.go)
- [Extensões backend](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/internal/server/extensions.go)
- [MCP](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/internal/server/mcp.go)
- [Mapa e estado](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/internal/store/store.go)
- [Mapa de processos](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/internal/store/maps.go)
- [UI e rotas](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/frontend/src/components/observa-app.tsx)
- [HTTP Logs Loki](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/internal/server/logs.go)
- [Fonte e consultas](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/frontend/src/lib/api.ts)
- [Arquitetura storage](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/deploy/storage/ARCHITECTURE.md)
- [Compose](https://github.com/ruanmonteiro19/devops-automation/blob/feature/agents-gateway-i18n/observa-lite/compose.yaml)

#### Alertas de apresentação enganosos a corrigir
1. `internal/server/extensions.go` anuncia `service_map: true` em `/api/v1/capabilities`, embora a versão atual retorne `service_map` vazio sem instrumentação APM. Separar `process_map_available` de `apm_service_mapping_supported`.
2. `frontend/src/components/observa-app.tsx` contém muitas rotas que caem no `FuturePage`. Menu não é funcionalidade implementada.
3. A descrição de `MCP` e `list_services` ainda pode sugerir descoberta de serviços automática; atualmente o Service Mapping permanece vazio por desenho.
4. `README.md` descreve fases históricas já superadas e outras funcionalidades como se fossem atuais. Criar README de estado por release com capacidades geradas da API real.
5. `frontend/src/components/observa-app.tsx` renderiza painel HTML em `iframe sandbox=""`, enquanto a proposta de HTML Graphics promete JavaScript editável; verificar comportamento antes de afirmar suporte a JS.
6. `internal/store/store.go` reescreve o JSON completo em cada Ingest e em várias mutações; haverá limite severo para grandes inventários.
7. O backend tem estado no tempo presente mas não séries históricas de métricas/spans. Não se pode calcular anomalia/SLO a partir de snapshots estáticos.
8. `cmd/observa-gateway/main.go` é um relay, não é mecanismo completo ActiveGate; `routing.go` não tem fila durável ou ACK fim a fim.

## Matriz completa

### 01. Agentes, sistemas operacionais e descoberta — P0

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Descoberta Linux via /proc e sockets** | BÁSICO — Host, PID, RSS, sockets, relatório | Repetir em RHEL/Oracle Linux/Ubuntu/Debian/Alpine, com permissões diversas |
| **Agente Windows via PowerShell/CIM** | BÁSICO — Processos e conexões TCP | Windows Server 2019/2022/2025, permissões, fallback e serviço persistente |
| **CPU, memória, disco, IO, rede do host** | AUSENTE — Agente e armazenamento de séries | Dados históricos e alertas por host |
| **Process Group e identificação estável de instâncias** | PARCIAL — Agrupamento passivo por nome | Evitar IDs instáveis, múltiplos serviços no mesmo PID |
| **Descoberta opt-in por host, grupo e namespace** | PARCIAL — Flag do Agent desativa processos | Políticas persistentes por entidade, efetivas no Agent |
| **Políticas de Full-stack/Infra/Discovery/Off** | SÓ INTERFACE — Modo discovery funcional; outros bloqueados | Todos os modos atuam de fato |
| **eBPF/Linux e ETW/WFP/Windows** | AUSENTE — Sensores específicos sob autorização | Fallback robusto, privilégios mínimos e overhead medido |
| **Instalação, upgrade, rollback e desinstalação Agent** | PARCIAL — Binários/scripts iniciais | Installers suportados, sem perda de estado e assinatura Windows |
| **Agente remoto e telemetria própria de saúde** | AUSENTE — Controle de versões e métricas do Agent | Lag, perda, heartbeat, buffer, atualização segura |

### 02. Gateway, Network Zones e Host Groups — P0

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Gateway TLS com autenticação de ingestão** | BÁSICO — Relay Go em 18443 com mTLS opcional | Teste certificado SAN/CA corporativa e rotação |
| **Cadastro de destinos HTTPS no servidor** | BÁSICO — APIs admin e UI recém implementadas | CRUD, persistência, permissões e validação fim a fim |
| **Endpoint connectioninfo por Network Zone** | BÁSICO — Rotas ordenadas com fallback explícito | Validação em duas redes segregadas |
| **Failover por tentativa em erro 5xx/rede** | PARCIAL — Agent tenta endpoint seguinte | Teste fim a fim, timeout, backoff e recuperação |
| **Heartbeat/registro e inventário de gateways reais** | AUSENTE — Só destinos cadastrados, não saúde | Gateway anuncia versão, capacidades e disponibilidade |
| **Network Zones estritas, preferências e isolamento** | PARCIAL — Filtro por nome e fallback | Aplicação de políticas de zona com fail-closed |
| **Host Groups como política e administração** | PARCIAL — Atributo no inventário | CRUD, hierarquia, escopo, tags e acesso |
| **Buffer durável e store-and-forward** | AUSENTE — Falha de comunicação descarta lote | Limites, compressão, fila em disco e retenção |
| **IDs de lote, ACK, deduplicação e replay** | AUSENTE — Sem exactly-once | Evitar eventos duplicados mesmo após timeout |
| **Identidade e rotação de credenciais por Agent** | AUSENTE — Token compartilhado | Certificados individuais, expiração, revogação e renovação |
| **Alta disponibilidade multi-Gateway / multi-Server** | AUSENTE — Um servidor lógico | Múltiplas réplicas, teste de falhas sem perda não controlada |

### 03. Ingestão OpenTelemetry e APM — P0

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **OTLP HTTP/protobuf e gRPC para traces** | AUSENTE — Sem receiver de spans | Serviços Java/.NET enviam spans com auth, throttling e TLS |
| **OTLP metrics e logs** | AUSENTE — Sem receiver central | Validação de schema, quotas e filtros |
| **W3C Trace Context e ligação pai/filho** | AUSENTE — Sem modelo de spans | Propagação e spans relacionados entre serviços |
| **Identidade lógica service.name/namespace/instance** | AUSENTE — Apenas processos passiveis | Instâncias agrupadas em uma única entidade serviço |
| **Descoberta de endpoints e Service Detection rules** | AUSENTE — Sem catálogo de endpoints HTTP/gRPC | Naming, merge/split, exceções e regras auditáveis |
| **RED: Request, Error, Duration por serviço** | AUSENTE — Sem métricas APM | Medidas reais no intervalo escolhido |
| **Auto-instrumentação Java** | AUSENTE — Sem autoinjeção | Agent Java OpenTelemetry, compatibilidade, rollback |
| **Auto-instrumentação .NET e IIS** | AUSENTE — Sem profiler instalado | ASP.NET/.NET/IIS suportados com validação por versão |
| **Node.js/Python instrumentados** | AUSENTE — Sem instrumentação | Traces, erros e downstream |
| **SQL/JDBC/DB spans, chamadas externas e mensageria** | AUSENTE — Apenas inferência de sockets | Dependências L7 com status, tempos e atributos |
| **Profiling, exceptions e análise de runtime** | AUSENTE — Sem coleta | Opt-in por tecnologia; overhead e dados sensíveis controlados |

### 04. Topologia, Smartscape e mapas — P0

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Process Mapping passivo e Network Mapping IP** | BÁSICO — React Flow e grafo de sockets | Conferir direção, duplicação e limites de 1.000 nós |
| **Service Mapping baseado em traces reais** | AUSENTE — API retorna vazio sem instrumentação | Nós e relações gerados somente por L7/OTLP |
| **Smartscape 5 camadas e navegação cruzada** | AUSENTE — Um mapa React Flow por tipo | Aplicações, Serviços, Processos, Hosts, Data Centers |
| **Layout por força/hierárquico e agrupamento** | AUSENTE — Posições calculadas em grade 3 colunas | Layout legível, cluster e performance grandes grafos |
| **Drill-down entidade, chamada, requisição e SQL** | PARCIAL — Painel lateral básico | Links a logs, traces, métricas, eventos e histórico |
| **Mapa temporal e diff de dependências** | AUSENTE — Sem histórico de arestas | Comparar T0/T1 por período |
| **Detecção de links externos, origem e confiança** | PARCIAL — Edges TCP observadas | Evidências por timestamp e classificação L7/manual |
| **Saúde e propagação de impacto entre camadas** | AUSENTE — Status host baseado em LastSeen | Health models e causa/impacto com regras |
| **Topologia física SNMP/LLDP/CDP** | AUSENTE — Network Mapping não é físico | Switches, interfaces, vizinhança e estado real |

### 05. Templates Zabbix-like e descoberta LLD — P1

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Checks HTTP e TCP básicos** | BÁSICO — Runner e criação | Histórico, timeout e classificação |
| **Itens de host e métricas / tipos de item** | AUSENTE — Apenas template com um check | Item keys, interface, tipo e status |
| **Triggers, severidades e expressões** | AUSENTE — Somente falha de check | Parser, dependências e janela temporal |
| **Macros de host, template e LLD** | AUSENTE — Sem macros | Escopo, herança e expansão segura |
| **Preprocessing, dependent/calculated items** | AUSENTE — Sem pipeline | JSONPath, regex, multiplicadores, descarte e teste |
| **LLD rules e prototypes** | AUSENTE — Sem modelo de discovery rule | Criar/remover itens, triggers, hosts e gráficos a partir de dados reais |
| **Herança de templates, vinculação e tags** | AUSENTE — Template isolado | Template auxiliar e múltiplos links |
| **Importação/exportação JSON/YAML e versão** | AUSENTE — Exemplos manuais JSON | Validar schema, preservar configuração, rollback |
| **Manutenção e janelas de supressão** | AUSENTE — Sem maintenance API | Alertas não disparados durante janela autorizada |
| **Sintéticos e verificações ativas distribuídas** | AUSENTE — Só HTTP/TCP locais | Multi-localidade, TLS check, certificados e alertas |

### 06. Integradores e Data Sources Grafana-like — P1

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Prometheus / PromQL instantâneo** | BÁSICO — Consulta HTTP a datasource | Teste rede, auth e formatos; query_range |
| **REST/JSON** | BÁSICO — GET + selector limitado | Paginação, extratores, esquemas e credenciais |
| **Loki / LogQL sob demanda** | BÁSICO — Queries restritas | Visualização e contextos correlatos |
| **VictoriaMetrics, Thanos e métricas históricas** | AUSENTE — Sem TSDB local | Query range e downsampling |
| **Tempo/Jaeger e traces** | AUSENTE — Sem consulta de spans | Trace IDs e links Service Mapping |
| **Elasticsearch/OpenSearch** | AUSENTE — Só catálogo | Query DSL e RBAC |
| **Postgres/MySQL/SQL Server/Oracle consultas** | AUSENTE — Só catálogo | Credenciais cofre, read-only e timeout |
| **Zabbix API + importação templates/events** | AUSENTE — Só catálogo | Autenticação e sincronismo incremental |
| **InfluxDB, GraphQL, CSV/XML e fontes plugin** | AUSENTE — Nenhum adapter | Schemas e testes por provedor |
| **Cloud AWS/Azure/OCI** | AUSENTE — Catálogo estático | Discovery e autenticação segura |
| **Datasource provisionamento/test connection** | PARCIAL — Cadastro simples | Teste sem gravar segredo e métricas do connector |
| **Plugin API para conectores externos** | AUSENTE — Conectores compilados no backend | SDK, manifesto, versionamento e sandbox |

### 07. Logs e dados leves — P1

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Consulta Loki LogQL** | BÁSICO — GET /api/v1/logs; limites | Filtros, labels e range queries |
| **Agent/Collector para logs Linux/Windows** | AUSENTE — Nenhuma coleta automática | File/journald/eventlog com filtros e posições |
| **Parsing, redaction e logs estruturados** | AUSENTE — Sem pipeline | Mascaramento e labels de baixa cardinalidade |
| **Link logs ↔ traces ↔ serviços ↔ problemas** | AUSENTE — Não há spans nem IDs correlatos | Do log ao trace e vice-versa |
| **Gestão de retenção, cota e cardinalidade** | PARCIAL — Exemplo Loki com retenção | Alertas de disco e limites de volume/burst |
| **Log Explorer refinado** | PARCIAL — Formulário LogQL + tabela | Timeline, live tail com limites, contexto e salvar consultas |
| **Armazenamento de métricas sem Elastic** | AUSENTE — JSON de metadados, sem TSDB | SQLite config + motor timeseries opcional |
| **Traces com Tempo e retenção independente** | AUSENTE — Sem receiver/store | Opt-in, sampling e link de trace |

### 08. Dashboards e editor Grafana-like — P1

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Criar dashboards stat e HTML simples** | BÁSICO — Persistência JSON + UI básica | Renderizar valores de queries realmente |
| **Layout drag & drop e resize** | AUSENTE — Grid fixo de cards | Posicionar/salvar grid responsivo |
| **Painéis timeseries/bar/gauge/table/heatmap** | AUSENTE — Apenas HTML/stat rudimentares | Queries reais, histórico, unidades e legendas |
| **Queries por painel + transformações/overrides** | AUSENTE — Texto da query pode virar visual | Editor de query, join, threshold e override |
| **Variáveis, filtros, rows e repeat panels** | AUSENTE — Sem modelagem de variáveis | Painéis dinamicamente repetidos |
| **Importar Grafana Classic JSON** | AUSENTE — Não há parser/conversor | Compatibilidade de gridPos e datasource mapping |
| **Exportar/provisionar dashboards e biblioteca** | AUSENTE — Sem export/Library Panels | Round-trip e versionamento |
| **HTML Graphics com JS isolado e bindings** | PARCIAL — iframe sandbox e HTML/CSS; JS não comprovado | Eventos init/render; isolamento contra secrets/DOM |
| **Dashboard folders, controle de acesso e versionamento** | AUSENTE — Só lista simples | RBAC e version history |
| **TV/Kiosk, auto-rotacionar e compartilhar** | SÓ INTERFACE — Rota Modo TV cai em FuturePage | TV funcional, layout 1920×1080 e refresh |

### 09. Incidentes, RCA e observabilidade assistida — P1

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Eventos de falha HTTP/TCP** | BÁSICO — Modelo Event e template runner | Histórico claro por serviço |
| **Alertas por métricas, logs, traces** | AUSENTE — Sem motor unificado | Threshold, baseline e anomalias |
| **Deduplicação e correlação temporal/topológica** | PARCIAL — Dedup estreito de falhas de check | Agrupamento e relacionamento com evidências |
| **Incident lifecycle, assignment, ack, comments** | AUSENTE — Modelo Status muito limitado | Aberto, reconhecido, resolvido, comentário e dono |
| **RCA guiada por evidência** | AUSENTE — Nenhum motor RCA | Causa provável x comprovada e grafo causal |
| **Impacto, root cause e Service Backtrace** | AUSENTE — Sem traces | Chamada downstream, causa e dependentes |
| **Anomalias, baselines e análise de mudança** | AUSENTE — Sem série temporal histórica | Métricas e deploy events |
| **SLO/SLI, error budget e burn-rate** | SÓ INTERFACE — Rota SLO cai em FuturePage | SLO calculado, tendências e alertas |
| **Integração ITSM/webhooks/workflow de acionamento** | AUSENTE — Menus apenas | Conector, retry, dedup e auditoria |

### 10. Kubernetes, OpenShift, containers e cloud — P1

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Leitura de processos em namespace host Docker Linux** | BÁSICO — container Agent com pid/network host | Identificação de container ao invés de PID genérico |
| **Inventário Kubernetes API/CRDs/pods/workloads** | AUSENTE — Menu de infraestrutura | RBAC read-only e clusters múltiplos |
| **DaemonSet/Operator e opt-in por namespace** | AUSENTE — Sem manifest do Operator | Install/uninstall e políticas |
| **Métricas pod/container e logs K8s** | AUSENTE — Sem scraper | CPU/mem/oom/restarts com namespaces |
| **Kubernetes service-to-service e deployment events** | AUSENTE — Sem API | Correlacionar workload, versão e rollout |
| **OpenShift deployment e projetos** | AUSENTE — Sem conector | Permissões e workloads reais |
| **AWS/Azure/OCI discovery** | AUSENTE — Catalogadas como futura | Inventário e telemetria por conta/tenant |

### 11. Redes, equipamentos e bancos de dados — P1

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Network Mapping de fluxos IP** | BÁSICO — Socket TCP pelo Agent | Não se confundir com rede física |
| **SNMP v2c/v3 e IF-MIB** | AUSENTE — Sem polling SNMP | ifOperStatus, bytes, index e nomes |
| **LLDP/CDP, interface discovery e SD-WAN** | AUSENTE — Sem camadas L2/L3 | Vizinho real, interfaces e links |
| **Bancos Oracle, PostgreSQL, SQL Server e MySQL** | AUSENTE — Catálogo apenas | Templates saúde/locks/sessions/queries/tablespaces |
| **JMX, WMI/PerfCounter, WinRM** | AUSENTE — Windows Agent só PID/socket | Regras e coleta limitadas por permissão |
| **Database dependency e SQL statement tracing** | AUSENTE — Sem APM SQL | Span SQL, latência e errors |
| **Certificados TLS e DNS monitoramento** | AUSENTE — HTTP monitor básico | Vencimento, SAN, resolução e validação |

### 12. Experiência digital, Synthetic e RUM — P2

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **HTTP synthetic multi-localidade** | AUSENTE — Runner HTTP/TCP não é plataforma synthetic | Monitores, agentes locais e histórico |
| **Browser Synthetic transactions** | AUSENTE — Sem browser runner | Scripts e waterfall |
| **Real User Monitoring (RUM) browser/app** | AUSENTE — Sem SDK ingest | Apdex, web vitals, erros e geografia |
| **Session Replay com consentimento** | AUSENTE — Nenhum recorder | Mascaramento, RBAC e retenção |
| **Mobile observability, crash e user sessions** | AUSENTE — Sem coleta | SDK e privacidade |
| **Experiência digital e funis** | AUSENTE — Sem eventos RUM | Jornadas agregadas |

### 13. Hub de extensões, automações, IA e MCP — P1

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Catálogo de extensões com busca** | BÁSICO — 20 itens definidos no servidor | Inventário real e versões compatíveis |
| **Instalar, atualizar, habilitar e remover extensões** | AUSENTE — Catalogar não instala | Manifest assinada, permissões e rollback |
| **Extensões Python / script / SNMP em sandbox** | AUSENTE — Sem runtime de plugins | Limite recursos, isolamento e auditoria |
| **MCP Server read-only** | PARCIAL — JSON-RPC stateless + 5 tools | Testar clientes MCP reais e segurança |
| **MCP Client para fontes/integrações externas** | AUSENTE — Sem runtime MCP Client | Autenticação e permissões por ferramenta |
| **AI Assist/Agentic RCA com evidências** | SÓ INTERFACE — IA & RCA como FuturePage | LLM opcional, evidência, controles de custo |
| **Workflow no-code / eventos / retry** | AUSENTE — Sem worker | Execução persistente, ações aprovadas e auditoria |
| **Webhooks, ServiceNow, BMC Helix, Jira, GLPI** | AUSENTE — Menu genérico | Conectores reais e mapeamento incidentes |

### 14. Autenticação, segurança e governança — P0

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Admin token e Agent token** | BÁSICO — X-API-Key e Bearer | Rotação/revogação por credencial |
| **Login por usuário e sessões seguras** | AUSENTE — Token em sessionStorage | Login backend, cookies seguros e rate limits |
| **SSO OIDC/SAML** | SÓ INTERFACE — Tela informa falta | Integrar provedor OIDC com testes |
| **MFA** | SÓ INTERFACE — Sem fator backend | Via IdP ou TOTP/passkeys com enrollment |
| **RBAC/ABAC, multitenancy e escopos** | AUSENTE — Tudo token admin | Roles por entidade/ambiente/API |
| **Auditoria de mutações e acessos** | AUSENTE — Tela Auditoria como FuturePage | Log imutável/retido por ação |
| **Cofre de segredos e credenciais por integração** | AUSENTE — SecretEnv indica envvar do processo | Encryption/secret store e redaction |
| **HTTPS para a UI e control-plane** | PARCIAL — TLS no servidor opcional; VM em HTTP | Certificado autorizado e redirecionamento seguro |
| **SSRF, limites, DOS, isolamento de HTML** | PARCIAL — Timeout/targets privados e iframe | Pen-test, CSP e quotas por usuário |
| **Segurança de supply chain, SBOM, CVE, assinatura** | AUSENTE — Build de imagens sem atestação suficiente | Scan/assinatura e revisão de dependências |

### 15. UX, idioma e acessibilidade — P1

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Tema preto/verde predominante** | BÁSICO — CSS tokens de tema | Contraste e consistência em todas as páginas |
| **Navegação extensa por áreas** | BÁSICO — Menus com numerosas rotas | Reduzir vazios e apresentar estado real |
| **Português e seletor EN/ES** | PARCIAL — Tradução de navegação | Todos os formulários, erro e dados localizados |
| **Smartscape interativo e desempenho WebGL** | AUSENTE — ReactFlow com grade | Gráfico grande fluido e legível |
| **Modo TV 1920x1080 e mobile** | SÓ INTERFACE — Modo TV FuturePage | Auto-refresh, foco e kiosk |
| **Pesquisa global e filtros por contexto** | PARCIAL — Texto de busca no frontend | Search entities index, facets e ACL |
| **Acessibilidade WCAG e navegação teclado** | VALIDAÇÃO INCOMPLETA — Componentes Radix fornecem base | Teste com teclado/screen reader |
| **Ajuda contextual e tours de instrumentação** | SÓ INTERFACE — Ajuda FuturePage | Passos de configuração reais |

### 16. Persistência, operação e qualidade — P0

| Requisito | Estado identificado | Critério mínimo de aceite |
|---|---|---|
| **Inventário/configuração em JSON atômico** | BÁSICO — Persistência no estado local | Migration idempotent, backup e restauração |
| **SQLite/WAL metadados, índices e histórico** | AUSENTE — Documento de arquitetura apenas | Migrar estado sem perda/roll back |
| **Time series storage leve** | AUSENTE — Prometheus externo consultado | Retenção/quotas e queries históricas |
| **Armazenamento tracing/logs separados** | PARCIAL — Loki opcional, Tempo ausente | Não depender de Elastic |
| **Ingestão em lote, rate limit, compressão** | AUSENTE — POST JSON snapshot cada ciclo | Limitar disco, CPU, tráfego e objetos |
| **Docker build AMD64/ARM64** | BÁSICO — CI com imagens candidatas | Suporte e rollback comprovados na VM |
| **CI unit/integration/browser E2E** | PARCIAL — Go tests/Chromium básicos | Teste cada função, segurança e acessibilidade |
| **Testes de carga (100/1000/10000 hosts)** | AUSENTE — Sem benchmark representativo | p95, RAM, disk, taxa e latência |
| **Backup automático, retenção, upgrades** | AUSENTE — Backup manual | Restore test e migrações versionadas |
| **HA, rolling upgrade, disaster recovery** | AUSENTE — Instância única | Failover de backend e evolução de schema |
| **Instalador Linux cross-distro e Windows serviço** | PARCIAL — Linux systemd, Windows Scheduled Task | Sistemas homologados, pacote assinado e documentação |
| **Health de coletores, ingestão, storage e Gateway** | PARCIAL — GET /healthz básico | Readiness, saturation, atraso e alarmes |

## Implementação por marcos e critérios de bloqueio

### Etapa A — Fundamentos seguros (não bloquear toda a plataforma com UI)
- Inventário de entidades e modelo de dados escalável; persistência versionada, testes de migração/restore e API de capacidades honesta.
- Control-plane de Agents/Gateways com heartbeat, autenticação individual, TLS, Network Zone, Host Group, dedup, quotas e filas.
- Agente Linux/Windows realmente homologado para ambientes suportados, sem colocar apps existentes em risco.
**Gate de saída:** atualizar/restaurar sem perda, simular interrupção de Gateway e recuperar lote sem duplicações; teste de 48 h sem vazamento de memória e backup/restauração verificados.

### Etapa B — APM de verdade
- Receiver OTLP e armazenamento opcional; instrumentação opt-in Java/.NET + OTel context; serviço lógico separado de PID; RED, endpoints, traces.
- Real Service Mapping com evidências + tempo; correlacionar com host e Kubernetes.
**Gate:** dois serviços independentes (Java e .NET) emitindo tráfego, erro e SQL; traces/nós/arestas corretos e histórico navegável.

### Etapa C — Templates e infraestrutura
- Zabbix-like schema de items/triggers/macros/LLD, SNMP v3 IF-MIB/LLDP, bancos, kubernetes e dados de hosts.
- Históricos e retenção de métricas sem Elastic.
**Gate:** dois templates independentes e LLD descobrindo/removendo interfaces; Oracle/Postgres/SQL Server sob escopos de credenciais autorizados.

### Etapa D — Visualizações e investigação
- Smartscape em camadas com force layout, classificação e filtros, grafo histórico; Grafana-like dashboard builder e importador/exportador JSON com compatibilidade documentada.
- HTML Graphics executado em sandbox com política de conteúdo restrita e API de dados.
**Gate:** usar dashboards Grafana reais com import preview e painel compatível; 1.000 nós renderizados com desempenho medido, nenhuma aresta inventada.

### Etapa E — Incidentes/IA/Extensões/Segurança
- Problem lifecycle, RCA com fontes verificáveis, SLOs, integrações ITSM/webhooks, workflows, MCP completo e extensões instaláveis.
- SSO OIDC/MFA/RBAC, auditoria, secret store e quotas.
**Gate:** simular falha real, identificar impacto/causa provável com evidências, abrir/encerrar workflow idempotente, testar permissões de leitores e admins, testes de segurança.

### Etapa F — Experiência digital / escala e operação
- Synthetic Browser, RUM, sesssões quando justificadas, multiarch, HA, DR, pacote de upgrade, observabilidade interna, acessibilidade e i18n completo.
**Gate:** performance e segurança auditadas, instalação em distribuições declaradamente suportadas e rollback bem-sucedido.

## Política de entrega — sem fingir que terminou
1. Uma funcionalidade só pode receber **Implementada** se backend + UI + persistência + documentação estão coerentes.
2. **Testada** exige unitários, integração, erros/edge cases e, quando há UI, teste de navegador real.
3. **Homologada** exige cenário real autorizado, TLS/RBAC/retention/performance e rollback comprovados.
4. Features sem serviço real devem aparecer no frontend como não implementadas ou serem removidas do menu, nunca simuladas.
5. Toda PR precisa apontar contratos, fontes públicas, casos de teste, CI verde e evidências; não promover `qa` a `edge` sem autorização.
6. Preservar a `x-oc-grafana`: não alterar outros serviços, daemon Docker, firewall global ou volumes.

## Referências externas para comparação (não constituem prova de implementação)
- [Dynatrace Operator](https://github.com/Dynatrace/dynatrace-operator) e [OneAgent SDK](https://github.com/Dynatrace/OneAgent-SDK) para arquitetura de roteamento/instrumentação pública.
- [Dynatrace documentação](https://docs.dynatrace.com/) para comportamento de Smartscape, Problems, OneAgent, service detection, Grail, logs e RUM.
- [Grafana dashboards](https://grafana.com/docs/grafana/latest/fundamentals/dashboards-overview/) para editor, datasources e transformações.
- [Zabbix LLD](https://www.zabbix.com/documentation/8.0/en/manual/discovery/low_level_discovery) para descoberta, macros e protótipos.
- [Coroot](https://github.com/coroot/coroot) para estudo de serviço → app e estatísticas de dependências.
- As licenças são específicas de cada repositório; jamais copiar código GPL/AGPL sem avaliar obrigações.

## Conclusão
**O Observa Lite é um MVP de descoberta/processos com superfície UI maior que suas capacidades; a diferença para a plataforma desejada é substancial.** A prioridade é fechar os ciclos completos de telemetria e qualidade, não aumentar a quantidade de menus antes do backend.
