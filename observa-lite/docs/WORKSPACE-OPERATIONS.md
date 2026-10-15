# Vetra 0.4 — Frontends, satisfação e operações

## Frontends pelo agente

O agente Linux associa os sockets TCP em escuta aos processos de `/proc`. Para runtimes web reconhecidos, verifica `GET /` no loopback. Resposta 2xx com `Content-Type: text/html` e documento HTML produz um candidato no menu **Experiência digital → Frontends descobertos**. A interface permite criar uma aplicação RUM a partir desse candidato, informando a origem pública.

O teste integrado executou o agente real, descobriu um listener Node e conferiu a evidência recebida pelo servidor. O reconhecimento também usa o nome do executável quando o processo altera o seu nome. Metadados de executáveis Go podem identificar listeners Go.

Limites: até 20 portas por ciclo, 400 ms por requisição, 4 s no total, sem redirects, cookies ou credenciais. Somente portas observadas e pertencentes a processos elegíveis. Não descobre automaticamente virtual hosts que exigem Host/SNI próprio, páginas autenticadas, HTML disponível apenas em outros caminhos ou listeners inacessíveis pelo loopback. HTTPS exige certificado válido para o endereço consultado. Containers em outros namespaces de rede podem exigir agente naquele namespace.

`OBSERVA_DISCOVER_FRONTENDS=false` desativa as sondagens. `OBSERVA_DISCOVER_SOFTWARE=false` desativa o inventário de pacotes. `OBSERVA_DISCOVER_PROCESSES=false` desativa ambos. O modo legacy omite os campos novos.

**Descoberta não é instrumentação do navegador.** Depois de criar o frontend, instale o SDK ou configure explicitamente o proxy de injeção conforme [RUM-INSTALLATION.md](RUM-INSTALLATION.md). Não há alteração automática de nginx/Apache nem injeção em todos os runtimes pelo agente.

## Satisfação

RUM mostra a **satisfação estimada pelo Apdex** no agregado e por sessão. O valor exibido em percentual é o índice multiplicado por 100, não a porcentagem de pessoas que avaliaram positivamente. Carregamentos iniciais satisfeitos valem 1, tolerados 0,5, frustrados 0. Erros da página frustram a amostra. Sem amostra válida, aparece `—`. Navegações SPA sem duração real não geram tempos inventados. Isso não substitui pesquisa de satisfação.

## Notebooks e VQL 1

Em **Painéis → Notebooks**, crie células de consulta e anotações, execute cada consulta e salve o documento. Anotações são exibidas como texto, sem executar HTML. Resultados são recalculados ao executar; não ficam congelados no notebook.

VQL é uma linguagem própria, limitada, diferente de DQL e SQL. Fontes: `hosts`, `processes`, `services`, `spans`, `logs`, `metrics`, `rum`, `frontends`, `databases`, `vulnerabilities`, `events`.

```text
fetch hosts | fields name, os, last_seen | limit 100
fetch processes | filter runtime == "java" | sort rss_bytes desc | limit 20
fetch spans | filter duration_ms > 500 | summarize count(), avg(duration_ms) by service_name
fetch logs | filter body contains "timeout" | limit 100
fetch vulnerabilities | filter severity == "High" | fields package, version, id, scanned_at
```

Etapas: `filter`, `fields`, `sort`, `limit`, `summarize`. Comparadores: `==`, `!=`, `>`, `<`, `>=`, `<=`, `contains`. Texto entre aspas duplas; comparação respeita tipos. Agregações: `count()`, `avg(campo)`, `sum(campo)`, `min(campo)`, `max(campo)`, com agrupamento por um campo opcional. Caminhos aninhados usam ponto. Pipes dentro de texto entre aspas são preservados.

Limites: 8.192 caracteres, 12 etapas, 15.000 linhas de entrada, saída padrão 500 e máximo 1.000. Logs/métricas limitados a 5.000 registros na leitura de origem; retenção existente também se aplica. API aceita janela de 1 a 1.440 minutos, padrão 60; interface usa 60. Inventários representam o último estado conhecido, não uma fotografia histórica. `truncated` informa cortes do motor, mas não representa cobertura da retenção ou da consulta de origem. Sem joins, funções de DQL, Grail ou armazenamento analítico distribuído.

## Workflows

Em **Confiabilidade → Workflows**, configure até 10 etapas:

- Consulta VQL.
- Condição: continuar quando a consulta anterior retornar ao menos N linhas.
- Verificação de perfil PostgreSQL.
- Avaliação de perfil de software no OSV.

Execução manual ou intervalo de 60 a 86.400 segundos com agendamento ativado. O servidor verifica vencimentos no seu polling. Falhas interrompem as etapas seguintes; condição não atendida encerra com `skipped_remaining`. Verificações e avaliações de perfis também produzem histórico próprio. Uma edição só passa a valer após **Salvar**; o botão de execução usa a versão salva.

Timeout de 90 segundos, exclusão mútua por workflow/operação principal e limite compartilhado de concorrência. São retidas até 100 execuções, no máximo 8 MiB no histórico e 3 MiB por resultado. Reiniciar o servidor marca trabalhos em execução como interrompidos; não retoma nem garante execução exatamente uma vez. Não há fila distribuída, retry automático, cron, gatilhos de evento, shell, webhook ou scripts arbitrários nesta versão.

## Databases

O inventário combina processos conhecidos e atributos de banco dos spans OTLP. Esses registros indicam presença, **não** disponibilidade SQL. A verificação ativa desta versão atende **PostgreSQL**; MySQL, Oracle, SQL Server, Redis e MongoDB não possuem conectores SQL ativos nesta entrega.

Crie um perfil com o nome da variável `OBSERVA_DB_PRINCIPAL_DSN`. No Docker Compose, defina-a no `.env`, já carregado pelo serviço `server`:

```dotenv
OBSERVA_DB_PRINCIPAL_DSN=postgresql://usuario:senha@postgres.empresa:5432/aplicacao?sslmode=verify-full
OBSERVA_ALLOW_PRIVATE_TARGETS=true
```

Use credencial com permissão de conexão e leitura das métricas necessárias. Recrie o serviço após alterar variáveis. O DSN não é enviado pela interface nem persistido no perfil; somente o nome da variável. Conexões privadas exigem a opção acima. Unix sockets não são aceitos. TLS segue a configuração pgx/DSN.

O coletor abre transação somente leitura e executa consultas fixas para versão, banco atual, tamanho, sessões, commits, rollbacks, deadlocks, leituras e acertos do cache. Timeout de conexão 5 s, statement 5 s, coleta 8 s. São contadores cumulativos, não taxas calculadas. Pode executar manualmente ou por workflow. A integração foi exercitada com pgx no protocolo PostgreSQL e uma instância **PGlite WASM**, incluindo consultas reais às views do sistema; isso não certifica todos os deployments PostgreSQL de produção.

## Vulnerabilidades

O módulo importa **CycloneDX JSON com purls** ou **package-lock v2/v3**, aceita pacotes manuais e importa o inventário descoberto pelo agente. O agente Linux lê metadados de dependências Go e lockfiles de produção Node no diretório de trabalho, com até 10 processos inspecionados e 500 pacotes. Não executa binários, não percorre discos e não comprova quais bibliotecas estão carregadas em memória.

Ao executar, nomes, versões e ecossistemas são enviados a `https://api.osv.dev`. O coletor trata paginação por pacote, busca detalhes dos avisos e mostra severidade quando fornecida, resumo, identificador, referência e versões de correção citadas. Estas versões não são uma recomendação automática de upgrade para qualquer branch. Severidade ausente fica `Unknown`; aviso retirado fica `Withdrawn`.

Limites: lotes de 50, até 5 páginas por consulta, 500 correspondências e 100 avisos detalhados. Cortes e falhas nos detalhes aparecem como parciais. Falha de consulta fica como erro, nunca como inventário limpo. O dataset VQL usa a última avaliação bem-sucedida por perfil, com `scanned_at`; confira a data e o histórico antes de interpretar resultados.

**Não é o módulo de vulnerabilidade do Dynatrace**, não usa seu feed e não analisa exploração, alcançabilidade de métodos ou vulnerabilidades de código em runtime. Ausência de correspondências não comprova segurança.

Foram testados paginação, avisos, correções, respostas inválidas e cortes com respostas OSV controladas. A tentativa de consulta ao OSV público nesta sessão foi bloqueada pela rede do ambiente de desenvolvimento; portanto a validação externa não foi concluída. No servidor com acesso externo, crie um perfil de teste `npm / lodash / 4.17.20`, execute e verifique o retorno antes de confiar na integração.

## MCP para comandos de IA

`POST /mcp` aceita JSON-RPC stateless com `initialize`, `ping`, `tools/list`, `tools/call` e notificação de inicialização. Autenticação pelo token administrativo em `X-API-Key` ou Bearer. O cliente de IA deve suportar este subconjunto HTTP; compatibilidade com todos os clientes MCP não foi certificada. Não há modelo embutido, MCP Client, OAuth, RBAC, SSE ou sessão persistente.

Leituras: `list_hosts`, `list_services`, `service_dependencies`, `network_flows`, `list_incidents`, `list_frontends`, `list_databases`, `run_query`, `list_workspace`, `list_operation_runs`.

Escritas: `configure_workspace`, `configure_rum`, `configure_host`, `run_operation`. O padrão `apply: false` retorna uma proposta; `apply: true` aplica com validação do backend. Anotações MCP são dicas para o cliente, não substituem autenticação ou controle de acesso. Todos os portadores do token administrativo têm os mesmos poderes.

Exemplo de chamada para criar um notebook por uma IA conectada:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "configure_workspace",
    "arguments": {
      "apply": false,
      "object": {
        "id": "operacao",
        "name": "Operação",
        "kind": "notebook",
        "cells": [{"id": "hosts", "kind": "query", "content": "fetch hosts | fields name, os"}]
      }
    }
  }
}
```

Para aplicar a proposta, repita com `apply: true`. Para executar um perfil ou workflow, use `run_operation` com `object_id` e `apply: true`. A chamada retorna o ID da execução; consulte `list_operation_runs` depois. Foram testadas prévia sem escrita, aplicação, persistência e autenticação; isso não equivale a validar cada aplicativo de IA externo.

## Validação reproduzível

```bash
go test -race ./internal/... ./cmd/...
go vet ./internal/... ./cmd/...
npm --prefix frontend run typecheck
npm --prefix frontend test
npm --prefix frontend run build
node scripts/smoke-workspace.mjs /caminho/observa
```

O smoke aceita `VETRA_PGLITE_ROOT`, `VETRA_PLAYWRIGHT_MODULE`, `CHROMIUM_PATH`, `VETRA_CHROMIUM_ARGS` e `VETRA_LIVE_OSV=true` para ativar as verificações opcionais. Sem essas variáveis, não alega executar PGlite, navegador ou OSV público. Não exige Docker para os testes locais.
