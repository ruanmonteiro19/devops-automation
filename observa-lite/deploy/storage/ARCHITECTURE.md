# Arquitetura de armazenamento: mínimo necessário por padrão

Objetivo: manter a aplicação de descoberta e monitoramento leve, sem adotar indexação de conteúdo em uma base unificada pesada.

## Estado atual, implementado
- Inventário, configurações, fontes de dados, templates, dashboards e eventos: arquivo JSON local, com escrita atômica. Adequado apenas a MVP/lab pequeno; ainda NÃO é SQLite.
- Service Mapping e Network Mapping: derivados do inventário em memória, não criam índices textuais.
- Logs: não são coletados nem persistidos pelo Observa Lite. A API GET /api/v1/logs consulta exclusivamente Loki configurado e devolve respostas delimitadas.
- Métricas externas: Prometheus/JSON consultados sob demanda. Não há TSDB interno.
- Traces: ainda não implementados.

## Evolução planejada, NÃO implementada
- SQLite/WAL para metadados, configurações, inventário, regras, migrações e auditoria. Versionamento e backup antes da migração do JSON.
- VictoriaMetrics single-node ou Prometheus compatível para métricas, opcional, com default de retenção 30 dias e cardinalidade limitada.
- Loki como opção para logs; padrão de retenção 7 dias (configuração de lab em deploy/loki) com índice de labels de baixa cardinalidade e chunks compactados.
- Tempo como backend opcional para traces, default pretendido de 3 dias.
- Cotas por fonte, por host e por tenant; descarte seletivo, amostragem, filtros antes da ingestão, painéis de armazenamento e alertas de disco.
- Políticas de mascaramento/redação antes de enviar mensagens de log para qualquer coletor.

## Regras de design
1. Somente o catálogo e as capacidades necessárias são carregadas. Sem exigir Loki/VictoriaMetrics/Tempo para Discovery.
2. A retenção por tempo não é uma quota de bytes: aplicar limites de disco por volume, alarmes e ingestão.
3. JSON local atual não é adequado para milhões de conexões ou altas taxas. Não prometer escalabilidade inexistente.
4. Labels de índice fixos e de baixa cardinalidade: ambiente, aplicativo, servico, nivel, host. Trace/request IDs ficam nos metadados estruturados.
5. Logs e traces desligados por padrão; habilitação explícita por serviço, namespace e política de acesso.
6. Preferir consultar na origem sob demanda a duplicar dados.
