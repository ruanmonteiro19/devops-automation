# Injeção RUM e proxies Vetra

## O que esta entrega faz

| Componente | Implementação validada |
| --- | --- |
| Agente | Descobre listeners HTML e registra o cabeçalho de status de injeção observado. Exporta o módulo Node embarcado. |
| Node.js | Preload HTTP/1 insere o SDK em respostas HTML elegíveis, sem editar o código da aplicação. |
| Proxy de aplicações | Insere o SDK no HTML, independentemente do runtime atrás do proxy; múltiplos domínios, caminhos e backends. |
| Gateway de telemetria | Encaminha ingestão de agentes, OTLP e endpoints públicos do RUM, com TLS e opções de mTLS. |
| Interface RUM | Permite baixar o módulo Node e gerar configuração de rotas por aplicação. |

**Não é uma implementação idêntica ao OneAgent.** Não há attach automático em todos os processos, módulos nativos Java/.NET/IIS/Apache, eBPF para modificar TLS ou descoberta e alteração automática de serviços do sistema. O módulo Node requer configuração do startup e reinício; o proxy requer que o tráfego seja direcionado a ele. Aplicações em Java/.NET/PHP/etc. podem receber injeção pelo proxy, quando entregam HTML elegível por esse caminho. Isso não representa instrumentação nativa desses runtimes.

## Node.js, sem editar o código

Na aplicação RUM, abra **Instalação → Injeção no processo Node.js**, baixe `vetra-node.cjs` e coloque-o em um caminho de leitura do processo. Alternativamente, no host com o binário do agente:

```bash
observa instrumentation --directory /opt/vetra
```

O comando exporta o módulo embarcado e recusa sobrescrever uma instalação existente. Para atualizar, faça backup e use um diretório novo. Não altera services ou processes em execução.

No gerenciador do serviço, defina:

```bash
export VETRA_RUM_SCRIPT_URL='https://coletor.empresa/rum/vetra.js?app=portal'
export VETRA_RUM_HOSTS='portal.empresa'
export VETRA_RUM_EXCLUDE_PATHS='/login,/auth,/checkout,/payment'
export NODE_OPTIONS="--require=/opt/vetra/vetra-node.cjs ${NODE_OPTIONS:-}"
```

VETRA_RUM_HOSTS filtra o Host recebido pelo Node, sem confiar em X-Forwarded-Host. Se o ingress reescreve Host, ajuste a lista para o valor efetivamente recebido. Um processo Node dedicado pode omitir esse filtro; em serviço compartilhado, prefira rotas separadas no proxy.

Reinicie o serviço com essas variáveis. Em Docker, monte o arquivo somente leitura e defina as mesmas variáveis no container da aplicação. O `NODE_OPTIONS` deve ser adicionado à configuração existente, preservando outros agentes/preloads. Teste a composição com outros módulos de instrumentação antes de usar em produção.

Elegibilidade: HTTP/1, status 200, `Content-Type: text/html`, conteúdo UTF-8 compatível com `<head>`, corpo entregue em `response.end()` e cabeçalhos ainda não enviados. Respostas em streaming ou já iniciadas por `write()`/`writeHead()` passam intactas. HTTP/2 e frameworks com implementações próprias de resposta não são instrumentados por esse hook. O teste local foi executado no Node 24.19; não houve certificação de todos os frameworks e versões Node.

Limite de corpo e descompressão: 1 MiB. Suporta corpo direto, gzip e Brotli em memória. Para o corpo modificado, recalcula comprimento, remove validadores/digests e aplica `Cache-Control: no-store`. Entradas binárias, charset incompatível, CSP não aprovada, páginas excluídas e injeção já presente são preservadas. Não lê corpo de requisição, senhas, cookies ou credenciais para produzir telemetria.

O cabeçalho `X-Vetra-Injection: injected-node` indica inserção nessa resposta. Uma sessão real recebida é a evidência de coleta; cabeçalhos isolados podem ser emitidos por qualquer upstream e não comprovam instrumentação ou consentimento.

Para política CSP que já permita o SDK e o coletor, o operador pode definir `VETRA_RUM_CSP_APPROVED=true`. Isso **não** altera a CSP, não cria nonce e não faz uma política incompatível funcionar. O padrão é preservar e não injetar. Consentimento, mascaramento, exclusões e amostragem do SDK continuam definidos no RUM.

Rollback: remova apenas a referência ao módulo Vetra de `NODE_OPTIONS` e reinicie. Alternativamente, `VETRA_RUM_ENABLED=false` deixa o hook inerte após reinício. Outros preloads devem ser preservados.

## Proxy de aplicações

O proxy recebe tráfego HTTP da aplicação atrás do ingress/TLS. A configuração contém destinos fixos, não permite escolher o upstream por query string e não é um proxy HTTP aberto.

Em **RUM → Instalação → Proxy de aplicações**, informe um ou mais upstreams, revise CSP, escolha ativação da injeção e baixe o JSON. Junte as aplicações no mesmo array `routes` quando necessário. O exemplo está em `deploy/rum/routes.example.json`.

```bash
cp deploy/rum/routes.example.json deploy/rum/routes.json
# Edite host, upstreams e script_url com os endereços reais.
docker compose --profile rum up -d --build rum-proxy
```

A interface também permite baixar o JSON já com o ID e a origem da aplicação. Coloque-o em `deploy/rum/routes.json` antes de iniciar. O arquivo é ignorado pelo Git para preservar a configuração local. O diretório é montado inteiro para permitir substituição atômica do arquivo.

No Compose, `host.docker.internal` aponta ao host Docker. Um serviço que escuta somente no loopback do host não será acessível por esse nome; use o endereço alcançável da aplicação ou a mesma rede Docker e seu nome de serviço. Para o binário executado diretamente no host, `127.0.0.1` pode ser apropriado.

Porta padrão: `127.0.0.1:8081`. Defina `VETRA_RUM_BIND_ADDR` e `VETRA_RUM_PORT` conforme sua rede, e configure DNS/ingress/balanceador para a aplicação passar pelo proxy. Inicie validando uma aplicação de teste. O proxy não troca portas ocupadas nem redireciona tráfego automaticamente.

Execução direta:

```bash
vetra-injector --listen :8081 --config /etc/vetra/routes.json --max-inflight 8
```

Características e limites:

- Até 100 rotas e 16 backends por rota. Host exato tem prioridade sobre `*`; usa o caminho mais específico. Sem match retorna 404. O path_prefix seleciona a rota, mas não é removido do caminho enviado ao upstream.
- Round robin por rota. Falha de transporte suspende aquele backend por 5 segundos para as próximas requisições. Não repete a requisição que falhou, evitando duplicar POSTs. Não existe health check ativo nem garantia de entrega.
- Configuração recarregada a cada 5 segundos. JSON inválido mantém a última configuração válida. Arquivo limitado a 1 MiB.
- `injection_enabled: false` mantém o proxy e desliga somente a injeção. `disabled: true` remove a rota do roteamento.
- Somente HTML 200 sem Range/HEAD, attachment ou `Cache-Control: no-transform`. Localiza o `<head>` com tokenizer HTML, preservando comentários e scripts.
- Descompressão gzip e recompressão quando necessário; limite de 8 MiB no corpo comprimido e descomprimido. Outros encodings são preservados sem injeção. O transporte não pede compressão ao upstream.
- CSP permanece inalterada. Sem aprovação explícita, HTML com CSP de header ou meta não recebe inserção. Injeção preexistente não é duplicada.
- Mantém cookies e corpo das requisições da aplicação. Remove cabeçalhos Forwarded informados pelo cliente e gera X-Forwarded-Host/Proto/For a partir da conexão imediata. Não confia automaticamente em informações de outro ingress. Revise redirects e cookies da aplicação: o Host enviado ao upstream é o Host do destino configurado.
- Até 8 requisições concorrentes por réplica no padrão; excesso retorna 503 com Retry-After. Dimensione memória considerando buffers HTML/gzip; aumentar concorrência exige recursos.
- `/__vetra/health` confirma configuração carregada, não a saúde de todos os backends. Este caminho fica reservado em modo de rotas.

O modo antigo com `--upstream` e `--script-url` continua disponível para uma aplicação. O controle de concorrência também se aplica a ele.

## Gateway de RUM e telemetria

O gateway agora oferece apenas estes endpoints públicos de navegador:

- `GET /rum/vetra.js` — SDK completo, sem o antigo limite de resposta de 4 KiB.
- `GET /rum/config/{id}` — configuração, com CORS preservado.
- `POST` e `OPTIONS /rum/beacon/{id}` — beacons e preflight.

Continua exigindo token de agente para `/api/v1/ingest` e `/v1/traces`, `/v1/logs`, `/v1/metrics`. APIs administrativas e `/mcp` não são expostos. Cookies, Authorization e token administrativo do navegador não são encaminhados para os endpoints RUM; Origin e Referer são preservados para a validação central.

Configure o endereço HTTPS público do gateway no campo de instalação do RUM. O coletor central continua validando origens, consentimento, tamanho e limites. O gateway não armazena replay nem inventário; é stateless. Há até 32 requisições concorrentes por réplica no padrão e corpos limitados a 6 MiB; respostas RUM limitadas a 8 MiB.

O perfil opcional Compose `gateway` usa rede host Linux, TLS em `deploy/gateway/tls/tls.crt` e `tls.key` e upstream padrão `http://127.0.0.1:8080`. Ajuste `VETRA_GATEWAY_UPSTREAM` se a porta central for diferente. Destinos remotos exigem HTTPS. O certificado e a chave devem ser legíveis pelo UID 65532 do container; não publique chaves no repositório. Configure os arquivos e o endereço antes de iniciar:

```bash
docker compose --profile gateway up -d --build gateway
```

`VETRA_GATEWAY_LISTEN` padrão é `127.0.0.1:18443`. Para acesso externo, defina a interface desejada e use certificado válido para o DNS público.

mTLS opcional: `--client-ca` mantém a exigência de certificado para todas as conexões, como antes. Para compartilhar esse listener com navegadores sem certificado, o operador precisa passar **explicitamente** `--public-rum-with-mtls`: a conexão pública é aceita, mas as rotas de agente/OTLP ainda exigem uma cadeia de certificado verificada além do token. Outra opção é separar gateways para navegador e para agentes.

Escalabilidade: múltiplas réplicas de gateway e do proxy podem ficar atrás de balanceador. A configuração precisa ser distribuída entre as réplicas. Não existe descoberta automática de réplicas, fila durável, autoscaling ou central em HA; o servidor com JSON continua sendo de uma instância. Aumentar proxies não torna a persistência central distribuída.

## Validação desta entrega

- Go `test -race` e `vet` nos comandos e pacotes internos.
- 80 testes de interface e build TypeScript/Vite.
- Teste Node com HTML, JSON, gzip, Brotli, CSP, exclusões, tamanho, HEAD, streaming, writeHead, cookies e desativação.
- Proxy: roteamento, round robin, backend com falha, headers Forwarded, gzip, tokenizer HTML, configuração inválida e bypass.
- Gateway: SDK de 200 KiB sem truncamento, CORS, paths permitidos, ausência de encaminhamento de credenciais e mTLS por rota no modo explícito.
- Chromium real: aplicação Node sem SDK no código → preload → proxy → SDK e configuração pelo gateway HTTPS → consentimento → beacons/replay mascarado → RUM/Apdex → download do módulo e configuração.
- Recarga real de arquivo e desativação da injeção sem interromper encaminhamento.

```bash
node --test scripts/test-node-injection.cjs
go test -race ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
node scripts/smoke-auto-injection.mjs /caminho/observa /caminho/vetra-injector /caminho/observa-gateway
```

O último teste aceita `VETRA_PLAYWRIGHT_MODULE`, `CHROMIUM_PATH` e `VETRA_CHROMIUM_ARGS`. Sem Playwright, não afirma executar navegador. TLS usa certificado efêmero de laboratório com exceção de confiança apenas no teste. Certificados de produção devem ser válidos.

Docker não está disponível no ambiente de desenvolvimento desta sessão. Os binários foram construídos e executados e o YAML Compose foi analisado, mas as imagens e os perfis Docker novos não foram executados localmente. Nenhuma configuração do seu host ou de suas aplicações foi alterada remotamente.

Fontes de mecanismo: [Dynatrace — automatic injection](https://docs.dynatrace.com/docs/observe/digital-experience/rum/web-frontends/initial-setup/configure-auto-injection), [Node.js — NODE_OPTIONS/require](https://nodejs.org/api/cli.html). O código implementado é próprio; não incorpora o OneAgent.
