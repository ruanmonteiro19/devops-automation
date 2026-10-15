# Configuração APM — OTLP HTTP

O servidor recebe traces em `POST /v1/traces`, logs em `POST /v1/logs` e métricas em `POST /v1/metrics`, na mesma porta HTTP do Observa Lite. Não existe listener gRPC 4317 nesta versão. É necessário instrumentar as aplicações com SDKs/agents OpenTelemetry ou usar um Collector que já receba spans.

## Collector

Exemplo para um Collector já existente. Ajuste hostname/HTTPS à sua rede e mantenha a validação de certificados. A variável `OBSERVA_AGENT_TOKEN` precisa estar no ambiente do Collector. Use o token do agente, não o administrativo.

```yaml
exporters:
  otlphttp/observa:
    endpoint: https://observa.exemplo.local
    headers:
      X-API-Key: ${env:OBSERVA_AGENT_TOKEN}
    compression: gzip
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/observa]
```

O exporter acrescenta `/v1/traces` ao endpoint base. Preserve receivers/processors/exporters que já existam no Collector; o exemplo mostra somente o trecho relevante. Receiver `otlp` e processor `batch` devem estar definidos na configuração completa. Os mesmos exporter e autenticação podem ser usados em pipelines de logs e métricas. Métricas aceitas: gauge, sum e histogramas explícitos. Summary e exponentialHistogram são rejeitados explicitamente com HTTP 400; não são descartados silenciosamente.

O gateway Observa também encaminha `/v1/traces`, `/v1/logs` e `/v1/metrics`, preservando protobuf e gzip, com o mesmo token do agente.

## Consultas administrativas

Todas exigem `X-API-Key` administrativo. `from` e `to` são timestamps RFC3339; padrão última hora, intervalo máximo 24h.

| Rota GET | Conteúdo |
|---|---|
| `/api/v1/apm/services` | Serviços, namespace, instâncias informadas, RED, percentis e endpoints |
| `/api/v1/apm/timeseries` | Até 60 intervalos por serviço; máximo 200 serviços por volume de requisições |
| `/api/v1/apm/traces` | Traces; filtros `service` (ID lógico ou nome), `failed=true`, `limit` 1..200 |
| `/api/v1/apm/traces/{trace_id}` | Spans, eventos, atributos, parents ausentes; trace retido inteiro |
| `/api/v1/apm/map` | Arestas pai/filho entre serviços no período |

Na interface, abra Serviços, selecione um serviço e depois um trace. A seleção de um span mostra recurso, atributos e eventos/exceções. Erros & exceções lista traces com spans falhos; não implica falha em todos os serviços participantes.

## Interpretação

Contagens representam os spans retidos, sujeitos a amostragem, retenção e capacidade. Requisições são spans server/consumer; latência inclui o tempo do span, sem tentativa de calcular tempo exclusivo ou critical path. Exceção recebida como evento é exibida, mas não torna automaticamente falho um span marcado como sucesso. Instâncias são contadas apenas quando `service.instance.id` existe; zero indica que nenhuma identificação de instância foi informada.

Retenção lógica 24h; capacidade 20.000 spans/32 MiB; corpo máximo 4 MiB comprimido/descomprimido; span máximo 64 KiB; lote máximo 10.000. O estado é local e não deve ser tratado como armazenamento APM de escala empresarial. Expiração/capacidade podem remover partes de traces. Deduplicação usa trace_id/span_id, primeira observação retida prevalece.

Erros de autenticação 401, formato/codificação não suportado 415, corpo excessivo 413, conteúdo inválido 400, falha de persistência 500. Repita lotes após erros transitórios conforme seu Collector. Não há suporte a links assíncronos de spans, flags e metadados completos de scope nesta etapa.


## Análise de serviço, logs e métricas

- `GET /api/v1/apm/analysis?service=<id lógico>&from=&to=`: falhas, endpoints, chamadas de banco/externas/mensageria, instâncias e versões. Máximo 200 grupos por categoria e 5 traces de exemplo. Duração acumulada é soma de spans, com possíveis sobreposições.
- `GET /api/v1/telemetry/logs?trace_id=&service=&from=&to=&limit=`: logs recebidos por OTLP, separados da integração Loki existente. UI em Logs OTLP e no detalhe do trace.
- `GET /api/v1/telemetry/metrics?name=&service=&from=&to=&limit=`: pontos recebidos com tipo, unidade, temporalidade e dados originais. UI em Métricas. Inteiros permanecem strings para preservar precisão.

Logs: até 10.000 registros/16 MiB; métricas: até 50.000 pontos/16 MiB; ambos com retenção lógica de 24h, máximo 256 KiB por registro e corpo HTTP 4 MiB. Registros com conteúdo completo idêntico são deduplicados; eventos legítimos indistinguíveis também podem ser colapsados. Não use esse armazenamento limitado como arquivo de auditoria.

## SLOs e problemas por regra

`GET/POST /api/v1/slos`, `DELETE /api/v1/slos/{id}`. Avaliação com a mesma janela RFC3339 limitada a 24h. Disponibilidade por sucesso de requisições; latência por duração máxima. Sem amostra: valores indefinidos. Saldo em pontos percentuais (`resultado - meta`), burn rate na mesma janela; meta100 gera burn rate indefinido.

`GET/POST /api/v1/apm/rules`, `DELETE /api/v1/apm/rules/{id}`, `POST /api/v1/apm/rules/evaluate`, `GET /api/v1/apm/problems?status=active|resolved`. Até100 regras; métricas error_rate e p95_ms, janela1..60min, amostra mínima, severidade e ativação. Avaliação automática a cada30s enquanto o servidor roda. Sem amostra suficiente não fecha incidentes como recuperados. Máximo1.000 problemas e100 transições por problema. Não há envio de e-mails/webhooks nem RCA causal.

## Dashboards

Indicadores/séries APM usam `kind=apm-stat|apm-series`, `source_id=observa-apm`, `query` JSON com `service_id` e `metric`. Métricas: requests,error_rate,p50_ms,p95_ms,p99_ms,requests_per_second. CRUD pelo editor; exportação no formato nativo. Até100 dashboards,50 painéis por dashboard,1MiB por definição. Não há importação compatível com Grafana nem editor de grid com arrastar/redimensionar. HTML legado fica em sandbox sem scripts.

### Contexto e vínculos de spans

O receptor preserva `traceState`, `flags`, `links` (IDs, contexto, atributos e contagem de atributos descartados), `scope` (nome/versão/atributos) e os schema URLs do recurso/scope. Links usam IDs hexadecimais na API e não substituem `parent_span_id`. O trace de destino pode não ter sido recebido. A tela do span exibe esse contexto e links navegáveis.
