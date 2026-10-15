# Vetra 0.5 — Console, contas, downloads e armazenamento

## Correção do cadastro

O erro do print era `crypto.randomUUID is not a function`. Em HTTP por IP ou hostname não confiável, essa função pode não existir. Os formulários agora usam um gerador central de IDs, com `getRandomValues` quando disponível e fallback para identificadores de configuração. Esse helper não é utilizado para senhas ou sessões; segredos de autenticação são gerados no servidor com `crypto/rand`.

A correção cobre cloud, equipamento de rede, monitores, fontes de dados, dashboards, células e etapas de workflows. Foi exercitada no Chromium em um origin HTTP que realmente não expõe `crypto.randomUUID`; um interceptor de laboratório encaminhou as requisições para o servidor local, mantendo o origin HTTP para testar o comportamento do navegador. O cadastro foi persistido no banco e recarregado após reinício.

## Usuários e acesso

O primeiro início cria `admin` com senha temporária `12345`, somente quando ainda não existem usuários. O primeiro login exige trocar a senha por uma pessoal com 8 a 72 bytes. A credencial inicial não é recriada quando usuários já existem.

- Senhas com bcrypt; nenhum endpoint de contas retorna hash.
- Sessão aleatória de 256 bits, guardada no navegador em cookie HttpOnly e SameSite=Strict. Somente o hash do token da sessão é persistido. Expiração: 8 horas.
- Proteção CSRF em escritas por sessão, com verificação de Origin quando fornecido. Login tem limitação de tentativas por endereço de conexão.
- Perfil administrador: leitura e configuração. Perfil leitura: consulta, downloads e troca da própria senha; não altera recursos ou lista contas.
- Cadastro, edição, desativação, perfil e redefinição de senha em **Configuração → Usuários e acesso**. Não permite desativar ou rebaixar o último administrador ativo.
- Redefinições administrativas exigem troca no acesso seguinte. Trocar a própria senha revoga as sessões anteriores.
- Token administrativo anterior e token de agentes continuam válidos. O token administrativo mantém poderes completos; não recebe restrições de perfil.

Uma instalação já conectada por token continuará nesse modo. Use **Usuários e acesso → Sair** ou uma janela privada para entrar como `admin`. Não publique o token administrativo em comandos destinados a agentes. Para HTTPS terminado em um ingress, configure `VETRA_COOKIE_SECURE=true`; em HTTP de laboratório ele permanece false. SSO, MFA, tenants e políticas granulares por entidade não foram implementados.

## Componentes e downloads

**Configuração → Componentes e downloads** oferece executáveis reais, tamanho, plataforma e SHA-256:

- **Vetra Agent**: descoberta de hosts/processos/listeners e inventário de software. Agora tem entrada própria em `cmd/vetra-agent`, sem importar interface, servidor, SQLite ou PostgreSQL. O binário Linux AMD64 de teste ficou em aproximadamente 6,8 MiB, comparado aos aproximadamente 23 MiB do binário integrado anterior.
- **Vetra Relay**: gateway TLS de agentes, OTLP e RUM.
- **Vetra Relay Web**: proxy opcional de injeção HTML; é uma função diferente do gateway de telemetria.

Pacotes: Linux AMD64, Linux ARM64 e Windows AMD64. Foram compilados os nove executáveis; somente Linux AMD64 foi executado neste ambiente. O download do agente foi validado no navegador comparando os bytes e o SHA-256 do arquivo servido. Não há credenciais embutidas.

A imagem de servidor compila e inclui os arquivos em `/opt/vetra/downloads`. Para execução a partir do código:

```bash
./scripts/build-components.sh
export VETRA_DOWNLOAD_DIR="$PWD/downloads"
```

O servidor expõe somente nomes de artefatos permitidos, exige autenticação, ignora links simbólicos e não permite baixar arquivos arbitrários. Downloads não são os SDKs do Dynatrace nem cópias do OneAgent.

O agente passa a omitir processos de sistema e seus sockets no envio por padrão. Para investigação que precise deles, use `--include-system-processes` ou `OBSERVA_INCLUDE_SYSTEM_PROCESSES=true`. Os filtros do servidor e da interface permanecem.

## Navegação e frontend

A navegação lateral agrupa funções e remove atalhos duplicados de Apdex, replay/vitals, mapas de processo/rede/serviço e logs externos. As rotas antigas continuam acessíveis. Os mapas estão em **Visões do ambiente**; replay, vitals e Apdex permanecem nas abas de RUM. Logs do menu principal usam a ingestão interna OTLP; Loki é uma integração externa opcional, não uma dependência.

Frontends descobertos exibem:

- Configuração RUM associada pelo ID.
- Contagem real de sessões, acessos e sessões com replay nas últimas 24 horas.
- Ação para visitar a origem pública e gerar uma sessão de teste.
- Links diretos para acessos, replay e Experience Vitals com a aplicação pré-selecionada.
- Indicação de replay desativado e orientação de instalação/consentimento.

Descoberta do processo não cria gravações. Para ver replay, instale o SDK/preload ou direcione a aplicação pelo proxy, ative a função e autorize a captura no navegador. Telemetria de teste não é inventada para preencher a tela.

## Armazenamento nativo

O Compose principal agora usa **SQLite**, com migração inicial do JSON existente. Não precisa de Loki ou Tempo para guardar logs, spans, métricas, eventos RUM e replay.

Configuração:

```dotenv
VETRA_STORAGE_ENGINE=sqlite
```

O arquivo é criado ao lado do JSON: `state.json.sqlite`, com WAL e SHM auxiliares. O JSON original permanece como backup e **não continua sendo atualizado**. Para voltar ao modo JSON, exporte o estado atual; apontar novamente ao JSON antigo recuperaria somente aquela fotografia antiga.

Implementação:

- SQLite WAL, `synchronous=FULL`, transações e checkpoint automático.
- PostgreSQL opcional usando o mesmo contrato de registros. Timeout de conexão de 5 s e de statement de 15 s.
- Tabelas separadas de metadados e registros; hosts e resultados de integrações também são registros independentes.
- Payload sem índice textual de conteúdo; índices por tipo/tempo, serviço e trace ID.
- Gzip para payloads que se beneficiam da compressão, com writers reutilizados para reduzir alocações.
- Upsert somente quando o fingerprint muda; remoção dos registros expirados. Lote duplicado validado sem novas escritas.
- Manutenção periódica aplica retenção mesmo quando uma origem deixa de enviar dados. SQLite pode reutilizar páginas livres sem reduzir imediatamente o tamanho físico do arquivo.

**Armazenamento → Exportar backup** gera uma fotografia consistente, protegida por permissão administrativa. Inclui contas com hashes de senha, sessões e dados retidos, mas não as variáveis de ambiente com tokens e credenciais. Guarde também o `.env`.

### Limites atuais

O motor de consultas ainda lê a janela recente mantida em memória. Não é um banco colunar distribuído. A persistência incremental reduz regravações e disco; não elimina a serialização da janela na comparação de fingerprints nem os custos do agregador em memória.

Telemetria: até 24 horas, sujeita a cortes anteriores por quantidade e bytes. Limites existentes: 20 mil spans / 32 MiB, 10 mil logs / 16 MiB, 50 mil pontos de métricas / 16 MiB, 15 mil eventos RUM e 25 MiB de replay. Inventário, configuração e histórico operacional têm suas regras próprias. Não há object storage, Parquet, TraceQL, LogQL, DQL, cluster de escrita ou consultas históricas ilimitadas.

### Medição reproduzível

`TestCompressedStorageFootprint` cria 2.000 logs repetitivos:

| Medida | Resultado observado |
| --- | ---: |
| Snapshot JSON | Aproximadamente 5.680.779 bytes |
| Payloads SQL comprimidos | Aproximadamente 414.352 bytes |
| SQLite físico, com índices e arquivos auxiliares após checkpoint | Aproximadamente 999.424 bytes |

O payload representa cerca de 7,3% do JSON; o espaço físico medido ficou em cerca de 17,6%. É um cenário sintético repetitivo, não uma promessa de compressão ou desempenho para dados reais. Antes de checkpoint, WAL pode aumentar o uso temporário.

## Separar Web, API, banco, proxy e agente

O perfil `deploy/compose.components.yaml` tem serviços distintos:

| Serviço | Função | Porta padrão no host |
| --- | --- | --- |
| vetra-web | Interface e proxy de API | 127.0.0.1:18090 |
| vetra-server | Ingestão, consultas e controle | 127.0.0.1:18091 |
| vetra-db | PostgreSQL 17 | Sem porta publicada |
| vetra-proxy | Vetra Relay, opcional | 127.0.0.1:18444 |
| vetra-agent | Descoberta no host Linux | Sem listener |

As portas foram escolhidas para coexistir com a instalação do usuário em 18080 e os outros containers informados. Confirme disponibilidade antes de iniciar. Web e API continuam funcionando no modo integrado habitual; a separação é opcional.

Para uma instalação separada nova, a partir de `observa-lite`, acrescente uma senha de banco aleatória hexadecimal ao `.env` sem substituir os tokens existentes:

```bash
# Execute apenas se VETRA_DB_PASSWORD ainda não estiver definido.
printf '\nVETRA_DB_PASSWORD=%s\n' "$(openssl rand -hex 32)" >> .env

docker compose --env-file .env -f deploy/compose.components.yaml config --quiet
docker compose --env-file .env -f deploy/compose.components.yaml up -d --build
```

O Relay está em profile separado e requer certificados como documentado no guia de injeção. Não publica o banco. O DSN padrão usa senha hexadecimal; se optar por senha com caracteres reservados, forneça a codificação adequada no DSN. Para acesso externo ao Web, ajuste `VETRA_WEB_BIND` e use TLS no ingress.

### Migração de uma instalação existente

Não use o perfil separado com um banco vazio esperando que ele leia o volume antigo automaticamente. Primeiro exporte o estado por um administrador, pare as escritas da instalação antiga durante o corte, copie o JSON para o volume do novo servidor e defina `VETRA_MIGRATION_JSON` com o caminho do arquivo dentro do container. O PostgreSQL vazio importa esse JSON; depois reinícios usam o banco, não reimportam o arquivo. A aplicação recusa migrar a partir de um JSON legado quando detecta o SQLite ao lado, evitando usar um backup desatualizado por engano.

No laboratório, a exportação de SQLite foi importada no backend PostgreSQL, preservando usuários e integrações, e uma configuração nova sobreviveu ao reinício. O fixture foi PostgreSQL WASM/PGlite acessado por pgx no protocolo PostgreSQL, com modo simple_protocol para contornar a multiplexação de prepared statements do fixture. Não equivale a um teste de carga do PostgreSQL 17 em Docker.

A separação de containers não torna `vetra-server` multiwriter. Execute somente um escritor para esse store. Os Relays podem ter réplicas atrás de balanceador, conforme o guia anterior.

## Pesquisa de Loki, Tempo e Dynatrace

Consulta às fontes primárias:

| Produto | Mecanismo documentado | Aplicação na Vetra |
| --- | --- | --- |
| Loki | Indexa labels/metadados e comprime conteúdo em chunks; WAL; retenção/compactor | Conteúdo sem índice textual completo, compressão, WAL/transações, retenção. Os registros SQL não são chunks Loki. |
| Tempo | Agrupa spans por trace ID e usa blocos Parquet; escrita, consulta e manutenção separados | IDs e relações de spans preservados; persistência e consultas têm contratos separados. Não há implementação Parquet/TraceQL. |
| Dynatrace/Grail | Dados em buckets com políticas de retenção e tecnologia proprietária Datawarping | Tipos separados, limites explícitos e retenção. Não reproduz Datawarping nem Grail. |
| Dynatrace APM | Serviços, traces, relações, análise de chamadas e correlação | A Vetra mantém análise de serviços, RED, endpoints, traces, logs correlacionados, mapas, SLOs e regras já testadas. Não possui todas as detecções, RCA/Davis ou módulos nativos do fornecedor. |

Links:

- https://grafana.com/docs/loki/latest/configure/storage/
- https://grafana.com/docs/loki/latest/operations/storage/wal/
- https://grafana.com/docs/tempo/latest/reference-tempo-architecture/about-tempo-architecture/
- https://grafana.com/docs/tempo/latest/reference-tempo-architecture/block-format/
- https://docs.dynatrace.com/docs/platform/grail/dynatrace-grail/architecture
- https://docs.dynatrace.com/docs/platform/grail/organize-data
- https://docs.dynatrace.com/docs/observe/application-observability/distributed-tracing

Esses documentos descrevem mecanismos públicos. Não foi inspecionada a totalidade do código proprietário do Dynatrace, nem validada paridade integral de APM. A evidência REA anterior permanece em `docs/research/REA-RUNTIME-SUMMARY.json`, com seus limites. O código desta entrega é próprio; nenhuma implementação proprietária foi copiada.

## Validação e limites de ambiente

Testes Go e frontend, Chromium com origem HTTP sem randomUUID, cadastro de integrações, perfis de usuário, CSRF, troca de senha, download e hash de binário, links RUM, migração JSON/SQLite, lote duplicado sem escrita, retenção, SIGKILL e recuperação WAL, migração SQLite→PostgreSQL e reinício.

A coleta SNMP foi exercitada com um exporter HTTP de laboratório. Não houve credencial cloud real para certificar AWS/Azure/GCP externos nesta sessão. Perfis cadastrados sem credenciais não são apresentados como coleta bem-sucedida.

Docker não está disponível neste ambiente. Os binários foram compilados/executados e os YAML foram analisados; as imagens Web/PostgreSQL e o profile de componentes não foram iniciados aqui. ARM64 e Windows foram compilados, sem execução nesses sistemas. Não houve alteração remota na VM do usuário.


## Formulários de configuração

Os editores de integrações, monitores e RUM usam abas com painéis preservados, navegação por setas/Home/End e abertura da seção que contém um campo inválido. O padrão de linhas compactas também é compartilhado por regras APM, SLOs, dashboards, usuários, campos de editores antigos e Workspace Studio. Buscas e filtros continuam controles de consulta.

Obrigatório/opcional acompanha o atributo real `required` do controle; senha inicial é obrigatória ao criar usuário e opcional ao editar. Checkboxes não recebem indicação de obrigatoriedade artificial. A ajuda de rede mantém explícita a dependência do SNMP exporter; a interface não oferece parâmetros SNMP diretos inexistentes no backend.

A consultoria visual foi solicitada ao projeto Observa Lite UI no Lovable (mensagem `umsg_01m50ctd6tf5wt0khv5xx45fmz`). Foram aproveitadas as recomendações de alinhamento, controles sólidos de 36 px/44 px no mobile, ações visíveis e preservação das validações. A implementação e a verificação foram feitas neste repositório, não na cópia antiga do Lovable.
