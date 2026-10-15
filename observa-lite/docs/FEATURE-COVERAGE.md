# Cobertura funcional — Dynatrace como referência

Estado do código entregue em15/10/2026. **Implementado** significa comportamento básico testado nesta aplicação, não paridade completa ou homologação do produto Dynatrace. **Inventariado** significa apenas que o módulo apareceu no catálogo; não significa investigação integral de seus fluxos.

## APM e operação implementados

| Função | Entrega e evidência | Diferença ainda existente |
|---|---|---|
| Ingestão traces | OTLP HTTP JSON/protobuf/gzip; testes de schema/auth/limites | Scope (nome/versão/atributos), schemas, links entre spans, trace state e flags preservados; sem gRPC e sem todos os contadores de descarte |
| Serviços/endpoints RED | Spans server/consumer, identidade namespace/nome, rotas e percentis | Sem Service Detection configurável e sem métricas não amostradas |
| Séries por período | 60 buckets, até200 serviços por volume | Sem armazenamento temporal de escala |
| Service Mapping | Relações trace/parent entre serviços, contexto de parent e dependências externas identificadas por atributos de banco/mensageria/destino | Busca/tipos/layouts grade-horizontal-vertical e ocultação de isolados (máximo200 nós); sem topologia completa Grail/host/pod/cloud |
| Trace waterfall | Sequência, duração, parents ausentes, atributos/eventos | Sem profiling, stack de runtime ou critical path |
| Falhas e exceções | Agrupamento com HTTP/status/evento e traces de exemplo | Sem política de detecção de falhas equivalente a SDv2 |
| Banco | db.system/namespace/operation, contagens e duração | Sem SQL normalization/planos/sessões internas de DB |
| Outbound | Destino observado, métricas de spans, exemplos | Sem normalização de URLs configurável e análise RPC completa |
| Mensageria | Atributos messaging e duração | Sem acompanhamento de filas/broker e links assíncronos |
| Instâncias/versões | IDs/versões realmente recebidos | Sem deploy detector e inventário full stack |
| Logs | OTLP local + integração Loki separada; correlação trace | Sem OpenPipeline/DQL/pattern mining |
| Métricas | Gauge/sum/histogramas explícitos, tipo/unidade/exemplares | Sem summaries/exponential hist, PromQL local ou derivados automáticos |
| SLO | CRUD, disponibilidade/latência, meta/saldo/burn | Janela limitada24h amostrada; sem templates universais ou alerta próprio |
| Alertas e problemas | Regras error_rate/P95, amostra mínima, estados, evidências/timeline | Sem detecção automática de baseline nem RCA causal |
| Dashboards | CRUD, stat/séries reais, períodos, export nativo | Reordenação por botões, largura e importação JSON nativa; sem grid arrastável, import Grafana/Dynatrace ou DQL |
| Gateway | Relay autenticado dos3 sinais com tipos/gzip preservados | Sem fila durável/ACK/HA por sinal |
| Infra/processos | Discovery Linux/Windows anterior e Process Mapping separados | Sem OneAgent/eBPF/profiling/full stack automático |

## Investigação da versão nova

Observados Services com `Use latest features` ativado e tour do novo Explorer: topologia/call paths, segmentos por equipes/ambiente, filtros por campos Grail e investigação por endpoint. Observadas abas Database queries e Outbound calls; banco mostrou colunas query/db/system/errors/count/average/cumulative duration, mas sem resultados para o serviço amostrado. Outbound separa HTTP/RPC/Other.

SLO novo: lista de metas/warnings/período, exemplos disponibilidade/conversão/performance com14dias; criação desabilitada para a conta do playground. Não validamos a criação nesse tenant. Nossa UI tem configuração própria e amostra24h.

Smartscape novo: vistas Smartscape on Grail, infra, problemas, dependências de serviço, Kubernetes, AWS/Azure/GCP, AI Observability e network devices; navegação horizontal por chamadas e vertical por stack descritas na própria UI. Nosso mapa implementa apenas o subconjunto confirmado de spans/processos/IP.

## Catálogo visível no playground: áreas ainda sem equivalência

| Área | Módulos inventariados | Estado no projeto |
|---|---|---|
| Observe/explore | Notebooks, Investigations, Anomaly Detection, Reports, Data Explorer, Metrics Classic, Logs & Events Classic | Não equivalentes; apenas queries externas anteriores e regras explícitas novas |
| Infraestrutura | Clouds, Databases, Extensions, Kubernetes, AWS/Azure/GCP Classic, Cloud Foundry, Containers, Host Networking, Technologies/Processes, VMware | Conectores/full-stack dedicados ausentes; inventário passivo anterior permanece |
| Software delivery | Workflows, Site Reliability Guardian | Ausentes; SLO é somente o módulo descrito acima |
| APM avançado | AI Observability, Live Debugger, Multidimensional Analysis, Profiling & Optimization, Message Queues | Ausentes; spans e agrupamentos não substituem sensores desses módulos |
| Application Security | Vulnerabilities, Security Posture Management, Threats & Exploits | Ausentes |
| Digital Experience | Experience Vitals, Users & Sessions, Error Inspector, Synthetic, Opportunity Insights, Session Segmentation, Query User Sessions, Frontend/Web/Mobile, Session Replay, Custom Applications | Ausentes; checks HTTP/TCP anteriores não equivalem a browser/RUM/mobile |
| Business Observability | Business Flow, Compliance Assistant, Cost & Carbon Optimization, Explore Business Events | Ausentes |
| Gestão | Settings, Fleet Management, Discovery & Coverage, Hub, Deployment Status, Personal Access Tokens, User Settings, Credential Vault | Registro/rotas de agentes/gateways básicos; gestão corporativa de credenciais e identidade ausente |
| Outros/custom | Releases, Security Overview/Operations, Third-Party Vulnerabilities, Learn DQL, Sensitive Data Center, Cloud SRE Agents, Incident Quest | Sem implementação equivalente |

A existência dessa lista não comprova investigação minuciosa de todos os módulos. Paridade total ainda não foi atingida. Essa fronteira é explícita para impedir que menus e protótipos sejam tratados como funcionalidades concluídas.

## Continuação: contexto e vínculos OTLP

Links são evidências explícitas, inclusive entre traces distintos. A interface permite navegar ao trace referenciado; ele pode estar ausente por retenção ou amostragem. Esses links não são convertidos em parents nem em dependências comprovadas do mapa. Validado JSON/protobuf, IDs, persistência após reinicialização e isolamento de mutações. Flags são preservados integralmente, sem inferir amostragem global.

## Experiência do workspace

Reformulação de navegação, aparência, criação, ajuda e ações operacionais documentada em [WORKSPACE-EXPERIENCE.md](WORKSPACE-EXPERIENCE.md). Recursos sem implementação saíram do menu principal; isso não representa implementação dessas áreas.
