# Observa Lite

MVP de observabilidade em português com **agente Linux passivo**, descoberta de serviços, **Service Mapping**, **Network Mapping (fluxos IP)**, templates HTTP/TCP, Data Sources Prometheus/JSON, dashboards personalizáveis, HTML Graphics isolado e ferramentas MCP de leitura.

> **Estado:** protótipo funcional e validado localmente em Linux amd64. **Não é** substituto de Dynatrace/OneAgent, Zabbix ou Grafana em produção. Seu mapa de rede representa conexões IP, não topologia física. Não há instrumentação de runtime, eBPF, SNMP, LLD, distributed tracing, RCA avançado nem provedor LLM neste marco.

## Arquitetura

```
Linux /proc (somente leitura)
       │
       ▼
  observa agent  --HTTPS + token do agente-->  observa server
                                                 │
                                  ┌──────────────┴─────────────┐
                               Inventário                HTTP/TCP checks
                                  │                          │
                         Grafo de serviços              Incidentes
                         Grafo de fluxos IP                  │
                                  └──────────────┬────────────┘
                                         API / UI / MCP
                                               │
                                      JSON persistido
```

O agente descobre processos, RSS e sockets TCP lendo `/proc`. Associa sockets a processos através de descritores `/proc/<pid>/fd` e socket inodes. O servidor constrói o grafo a partir das observações, resolvendo o destino por endereço e porta quando este é monitorado. Relações com serviços externos são preservadas sem inventar identidade. O mapa não garante visibilidade de conexões efêmeras, HTTP TLS, código de resposta ou rastreamento distribuído.

## Início rápido (Linux, com Go 1.23+)

```bash
cd observa-lite
go test ./...
go build -o ./observa ./cmd/observa
export OBSERVA_ADMIN_TOKEN="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
export OBSERVA_AGENT_TOKEN="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
./observa server
```

Em **outro terminal**, usando o **mesmo** token `OBSERVA_AGENT_TOKEN`:

```bash
export OBSERVA_AGENT_TOKEN='<token gerado para o agente>'
./observa agent --server http://127.0.0.1:8080
```

Abra `http://127.0.0.1:8080` e informe `OBSERVA_ADMIN_TOKEN`. O backend escuta apenas em `127.0.0.1` por padrão. Para acessar uma máquina remota, prefira túnel SSH (`ssh -L 8080:127.0.0.1:8080 usuario@host`) ou proxy reverso HTTPS autenticado.

### Instalador (Linux x86_64 / aarch64)

O script instala binário estático (quando fornecido em `dist/`) ou compila localmente com Go, configura diretórios e, se houver systemd, registra e habilita dois serviços:

```bash
sudo ./scripts/install.sh all
sudo systemctl status observa-server observa-agent
```

A chave administrativa aparece na instalação inicial e fica em `/etc/observa/server.env`, acessível somente ao root. Para máquina somente com agente:

```bash
sudo env OBSERVA_SERVER='https://observa.seudominio' \
  OBSERVA_AGENT_TOKEN='<token remoto>' ./scripts/install.sh agent
```

Servidor sem systemd: o instalador fornece comandos para iniciar manualmente. Requer Linux com `/proc` montado, acesso ao processo/socket e architecture suportada. A visibilidade depende de namespaces, `hidepid`, permissões e políticas de segurança. **Não testado em todas as distribuições**.

### Configurar templates e fontes

Os templates são coletados pelo backend, a cada 10 segundos no mínimo, com concorrência limitada. Ative explicitamente `OBSERVA_ALLOW_PRIVATE_TARGETS=true` apenas para ambientes internos autorizados: por padrão, destino privado, loopback, link-local e multicast não são permitidos em consultas.

```bash
curl -X POST http://127.0.0.1:8080/api/v1/templates \
  -H 'Content-Type: application/json' -H "X-API-Key: $OBSERVA_ADMIN_TOKEN" \
  --data-binary @examples/template-http.json
```

Conectores implementados: `prometheus` (query instantânea PromQL) e `json` (GET REST + caminho de objeto como `$.status.count`). Campos `secret_env` referenciam variável **do processo servidor** contendo token de autenticação; segredos não são gravados no inventário. Consultas são somente leitura.

### Dashboards e HTML Graphics

O MVP oferece painéis `stat` e `html`. O painel HTML usa iframe com sandbox `allow-scripts`, sem `allow-same-origin`. O código HTML/CSS/JS editado por um administrador fica isolado do aplicativo; não trate como permissão de execução de código no servidor. Recursos Grafana completos (grid redimensionável, gráficos históricos, múltiplos painéis de visualização) permanecem no roadmap.

### MCP — somente leitura

O endpoint `POST /mcp` implementa um subconjunto stateless de MCP sobre JSON-RPC, com `initialize`, `tools/list`, `tools/call` e `ping`. A compatibilidade com clientes MCP de produção ainda não está certificada; usa token administrativo na versão inicial. Ferramentas disponíveis: `list_hosts`, `list_services`, `service_dependencies`, `network_flows`, `list_incidents`.

```bash
curl -s http://127.0.0.1:8080/mcp -H "X-API-Key: $OBSERVA_ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

## API

| Rota | Método | Acesso | Operação |
|---|---|---|---|
| `/healthz` | GET | Público | Status básico |
| `/api/v1/ingest` | POST | Token agente | Snapshot passivo |
| `/api/v1/snapshot` | GET | Token admin | Inventário, mapas, eventos |
| `/api/v1/templates` | POST | Token admin | Criar template |
| `/api/v1/templates/{id}` | DELETE | Token admin | Remover template |
| `/api/v1/datasources` | POST | Token admin | Criar fonte |
| `/api/v1/query?source=&expr=` | GET | Token admin | Consulta fonte |
| `/api/v1/dashboards` | POST | Token admin | Salvar dashboard |
| `/mcp` | POST | Token admin | MCP stateless somente leitura |

## Segurança e limitações

- Não instale na internet pública sem HTTPS e controles adicionais. Os tokens são necessários e distintos para o agente e para o administrador.
- `/proc` e sockets não dão métricas APM de métodos, consultas SQL, erros HTTP ou traces distribuídos. O tráfego TLS não é descriptografado.
- O agente **não** injeta código nem utiliza eBPF. A correlação implementada cruza `PID → socket → IP/porta → processo remoto` e registra desaparecimentos de serviços. Falhas HTTP/TCP de templates também geram incidentes.
- Armazenamento local JSON com escrita atômica é adequado para laboratório pequeno, **não** para milhares de hosts. Próximas etapas: PostgreSQL/ClickHouse, histórico de métricas, retenção e backup.
- Não há SNMP, LLD, auto-instrumentação Java/.NET, IA generativa, MCP Client, multiusuário/RBAC ou dashboards equivalentes ao Grafana.
- O código e a interface são originais; as referências Coroot, Checkmk, Zabbix e Grafana foram estudadas conceitualmente, sem incorporar código desses projetos.

## Roadmap

1. Persistência escalável, identidade de entidades e agentes eBPF com agregação local.
2. SNMPv3, LLDP e descoberta de interfaces / equipamentos; templates com macros e LLD.
3. OpenTelemetry OTLP ingestão, spans, logs, métricas e dependências L7.
4. Auto-instrumentação opcional Java/.NET sem modificar o código das aplicações.
5. Correlação temporal, SLO, regras de saúde, incidentes e RCA explicável.
6. Dashboard builder avançado, plugins, bibliotecas e importador Grafana.
7. Integrações MCP seguras com RBAC, agentes de IA e aprovação humana para ações.
8. Testes com distribuições Linux, namespaces Kubernetes, volumes de alta cardinalidade e arquiteturas adicionais.

## Licença

Código original deste repositório: MIT (ver `LICENSE`). Referências de terceiros possuem licenças próprias e **não** foram vendorizadas.


## Interface e Hub de Extensões (0.2.0 — branch de desenvolvimento)

A interface HTML embarcada ganhou uma barra lateral com **42 opções de navegação** agrupadas por área, incluindo APM, infraestrutura, monitoramento, dashboards, IA/MCP, extensões e administração. Os menus não suportados pelo backend indicam explicitamente que são recursos futuros.

Endpoints administrativos novos (exigem `X-API-Key`):
- `GET /api/v1/extensions`: catálogo de 20 extensões, distinguindo disponibilidade, configuração existente e recursos planejados.
- `GET /api/v1/capabilities`: capacidades reais, versão e estado atual da autenticação.

O Hub direciona extensões já suportadas aos formulários reais (Prometheus/JSON, checks HTTP/TCP, MCP e agentes), sem instalar executáveis arbitrários. Os conectores não implementados não podem ser ativados.

**Autenticação:** somente token administrativo. **SSO, MFA, RBAC e controle de usuários NÃO estão implementados nem protegendo esta versão.** A interface de segurança informa essa limitação. Não implantar este MVP exposto diretamente à internet.

### Redesign Lovable

Projeto de redesign em React/TypeScript (precisa de aprovação do plano no editor Lovable):
https://lovable.dev/projects/5ff82d39-2280-4b5e-99d9-634272c8e9d1

A integração da UI React ao binário Go ainda não foi realizada. O backend atual continua servido por HTML embarcado. Após a geração dos arquivos no Lovable, integrar build estático na pasta `internal/server/public` preservando os contratos REST existentes e os testes.



## Armazenamento leve e logs Loki (integração experimental)

A API GET /api/v1/logs consulta fontes Loki por LogQL, com autenticação administrativa e limites estritos (máx. 200 linhas; período máx. 24h; resposta limitada a 2 MiB). Os logs NÃO são copiados para o estado JSON do Observa Lite. Cadastro de Loki disponível em Data Sources, exploração em Logs. Coleta de logs não está implementada no agente Linux; é necessário enviar logs a Loki por um coletor externo apropriado. Experimente o Loki local SOMENTE em laboratório com o arquivo deploy/loki/compose.yaml. Retenção Loki configurada para 7 dias no exemplo, sem quota de tamanho em disco. Consulte deploy/storage/ARCHITECTURE.md para separação atual e proposta de SQLite/VictoriaMetrics/Tempo, ainda não implementados.


## Implantação por Docker Compose (Linux)

O projeto agora inclui **Dockerfile multi-stage** para React/Go, **Compose** para servidor+agente, e **Loki opcional** via profile, com volumes persistentes e limites de consumo.

```bash
cd observa-lite
./scripts/docker-init.sh
docker compose up -d --build server agent
docker compose ps
```

Interface no host Linux: http://127.0.0.1:8080 (binding de loopback por padrão).
Token de administração: `sed -n 's/^OBSERVA_ADMIN_TOKEN=//p' .env`.
Para Loki: `docker compose --profile logs up -d loki`. Não há coleta automática de mensagens ainda; é necessária uma integração externa de coleta.

O agente Docker usa namespace de PID/rede/UTS do host, executando como root com permissões de inspeção restritas, sem `privileged: true` e sem socket Docker. Esta configuração destina-se **exclusivamente a hosts Linux autorizados**, não ao Docker Desktop do Windows/macOS. SSO/MFA ainda não são suportados. Os metadados continuam em JSON local; **não** foram migrados para SQLite.

Veja o tutorial: [deploy/docker/README.md](deploy/docker/README.md). Build/testes e publicação experimental multiarch para GHCR em `.github/workflows/observa-lite-docker.yml` (na raiz do repositório).
