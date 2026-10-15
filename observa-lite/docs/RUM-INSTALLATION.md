# Vetra — instalação RUM, Replay, Apdex e Experience Vitals

## O que está disponível

Captura real em navegadores, CRUD de frontends, origens permitidas, consentimento, amostragem, identificação opcional por pseudônimo, sessões, erros técnicos, tempos/status de fetch e XHR, navegação SPA, replay web mascarado, Apdex por carregamento inicial e p75 de Web Vitals. Menu em seções compactas com a seção atual aberta. Os dados não são simulados no modo demo.

Não inclui SDK Android/iOS nativo, crash/ANR nativo, replay nativo, análise de vulnerabilidades de código em execução, geolocalização por IP, fingerprinting, correlação automática com spans backend ou instrumentação automática de todos os runtimes web. O proxy de injeção é um componente explícito, distinto do agente de descoberta de hosts.

## Atualizar a instalação existente

```sh
cd /opt/devops-automation
git pull --ff-only origin feature/dynatrace-apm-investigation
cd observa-lite
docker compose up -d --build server agent
```

Abra Experiência digital > RUM > Adicionar frontend. Cadastre a origem exata do site que o usuário acessa, por exemplo `https://portal.exemplo.com`, sem caminho ou wildcard. O ID da aplicação é público e não substitui autorização. O token administrativo serve somente ao painel/API de administração; nunca o inclua no SDK ou nos arquivos de um site monitorado.

O endereço do coletor precisa ser acessível pelos navegadores, não apenas pela rede Docker. Em produção use HTTPS com certificado válido. Uma página HTTPS não pode carregar o SDK por HTTP. O endereço localhost do seu notebook só funciona para navegadores no próprio notebook.

## Opção 1 — tag JavaScript

Copie a tag gerada em Instalação e inclua-a no head do site, preferencialmente antes dos scripts da aplicação:

```html
<script src="https://observabilidade.exemplo.com/rum/vetra.js?app=portal" crossorigin="anonymous"></script>
```

É possível incluir essa tag pelo mecanismo de implantação ou pelo servidor de páginas. O SDK possui build IIFE independente, sem dependência de CDN externa durante a execução. O build do projeto produz `frontend/dist/rum/vetra.js` e o servidor o incorpora à imagem.

## Opção 2 — proxy de injeção, sem editar arquivos da aplicação

No diretório observa-lite:

```sh
docker build -f deploy/Dockerfile.rum-injector -t vetra-rum-proxy .
docker run -d --name vetra-rum-proxy --restart unless-stopped \
  -p 8081:8080 vetra-rum-proxy \
  -upstream http://ENDERECO-DA-APLICACAO:PORTA \
  -script-url 'https://observabilidade.exemplo.com/rum/vetra.js?app=portal'
```

O endereço upstream é fixo e deve ser alcançável pelo container. Não use localhost para apontar para outro container ou para a VM. A porta 8081 deste exemplo recebe tráfego da aplicação monitorada; a Vetra permanece na porta 8080.

Para a captura acontecer, os usuários precisam acessar a aplicação por esse proxy ou por um ingress que o use como upstream. Não basta iniciar o container em paralelo. Revise cookies com Domain, redirects absolutos, autenticação, cabeçalhos de encaminhamento, cache e WebSockets com a aplicação real antes da mudança de rota. O componente não automatiza mudanças em DNS nem nos ingress existentes.

O proxy injeta uma única tag em respostas 200 `text/html` com head, preserva respostas JSON e outros recursos, remove validadores de cache da representação modificada e usa no-store para o HTML injetado. Pede conteúdo sem compressão ao upstream; se ainda receber uma resposta codificada, não altera o corpo. Não modifica HTML acima de 8 MiB. Respostas de streaming HTML só devem ser usadas após validação: a injeção exige leitura do documento.

Por padrão não injeta `/login`, `/auth`, `/checkout` e `/payment`; ajuste `-exclude-paths` conforme as rotas. O cabeçalho `X-Vetra-Injection` informa `injected`, `excluded`, `already-present` ou o motivo de não injetar.

## CSP e implantação

A política CSP da aplicação é preservada. Se houver CSP em header ou meta, o proxy deixa de injetar por padrão. Revise `script-src`/`script-src-elem` para a origem do SDK e `connect-src` para o coletor. Políticas com strict-dynamic, nonce, hash ou trusted types precisam de configuração específica: o proxy não gera nonce nem enfraquece essas políticas automaticamente.

Depois de validar a política, o operador pode iniciar o proxy com `-csp-approved`. Essa flag apenas permite tentar a injeção; não altera CSP e não garante que o navegador a aceite. Teste primeiro em uma rota controlada. Confira a aba Network para `vetra.js`, `rum/config` e `rum/beacon`; o beacon aceito retorna 202.

## Consentimento, dados e mascaramento

- Consentimento de RUM é exigido por padrão. O aviso opcional da Vetra permite iniciar a coleta sem editar o código da aplicação. A decisão é lembrada somente na aba. Integre-o ao gerenciador de consentimento do site se já existir um.
- Replay vem desligado. Habilitar a configuração não dispensa o opt-in separado do usuário. O checkbox de replay aparece no aviso quando permitido; a amostragem de replay também precisa incluir a sessão.
- DNT=1 e Global Privacy Control bloqueiam a coleta, inclusive após uma chamada de opt-in.
- IDs de sessão só são criados após opt-in. A sessão da aba pode ser reaproveitada em nova página se houver atividade nos últimos 30 minutos. Não há cookie persistente para reconhecer visitantes que voltam em outro dia. Uma página que permanece aberta não aplica um timeout duro de sessão.
- Inputs e alterações de inputs são sempre mascarados, incluindo o estado de seleção enviado pelo recorder. Campos editáveis e regiões bloqueadas nunca entram na lista pública de textos.
- O modo padrão mascara todos os textos. No modo allowlist, só o texto de elementos aprovados por seletor pode permanecer legível. Não use um seletor amplo sem revisar o conteúdo.
- CSS de layout calculado é preservado em um subconjunto. Scripts, atributos de evento, URLs de recursos, imagens, canvas, vídeos, fontes externas, estilos de conteúdo e iframes são removidos. Isso reduz a fidelidade visual e evita carregar recursos do site durante o playback.
- Não são capturados corpos ou cabeçalhos HTTP, valores de formulários, mensagens completas de erros, stack traces, query strings ou fragmentos. O caminho de URL é desativado por padrão; habilitá-lo pode revelar IDs ou dados incluídos na rota.
- O servidor não grava IPs do navegador nem seu User-Agent completo. Pode haver logs do seu ingress, proxy ou infraestrutura que precisam de política própria.
- As configurações do SDK são carregadas ao abrir a página. Alterações exigem recarregar páginas já abertas. Desabilitar o frontend bloqueia novos beacons no servidor. O servidor aplica a configuração atual de mascaramento na ingestão; ao consultar replay no modo all, reaplica o mascaramento de textos. Trocar seletores não torna possível reconstruir retroativamente quais seletores aprovaram um texto antigo.
- Mudanças de política não apagam automaticamente os registros já armazenados em disco. Para apagar dados antigos, use a exclusão da sessão ou do frontend. A exclusão não bloqueia novos dados enviados por uma sessão ainda ativa: revogue a captura também.

Integração opcional:

```js
window.Vetra.setConsent({ rum: true, replay: false });
window.Vetra.setConsent({ rum: true, replay: true });
window.Vetra.setConsent(false); // para observers/hook, descarta buffers e remove o ID local
window.Vetra.showConsent();
```

## Identidade

Com a opção de identificação habilitada, a aplicação pode informar um identificador interno opaco depois do login:

```js
await window.Vetra.identify('ID-INTERNO-OPACO');
window.Vetra.resetIdentity(); // logout
```

O SDK calcula SHA-256 antes de transmitir esse ID; o servidor deriva um pseudônimo com HMAC e segredo persistido localmente. Isso é pseudonimização, não prova de identidade nem anonimização irreversível. Não use CPF, email ou credenciais. Não existe associação automática entre sessões de dispositivos diferentes sem essa integração explícita. Os pseudônimos aparecem apenas após novos eventos; o ID bruto não é persistido pelo SDK.

## Métricas e interpretação

- Web Vitals usa web-vitals 5.3.0: LCP, INP, CLS, FCP e TTFB. A disponibilidade depende do navegador e da atividade. INP pode não existir sem interação válida. A API faz upsert do valor final por aplicação/sessão/documento/métrica, evitando contar cada atualização da mesma métrica como uma pessoa diferente.
- O p75 é calculado nas amostras válidas recebidas no período. O cálculo não agrega visitantes sem telemetria nem interpreta ausência como zero.
- Apdex considera carregamentos iniciais medidos pela Navigation Timing API. Satisfeito até T, tolerável até 4T, frustrado acima disso ou com erro JavaScript observado naquela página. O valor é `(S + Tolerável/2) / N`; sem N é indefinido. A mudança de T recalcula a consulta com o novo limite.
- Navegação SPA gera eventos de navegação e novos segmentos de replay. Não é atribuído um tempo zero de renderização para fabricar um bom Apdex; medir conclusões de renderização assíncrona de cada framework ainda exigiria instrumentação adicional.
- Fetch/XHR registram duração, status e erro técnico. Não há propagação automática de traceparent nem vínculo comprovado com o backend APM.

## Limites atuais

Até 50 frontends, 20 regras por categoria, 100 eventos/10 chunks por lote, corpo máximo 2 MiB e 600 beacons por minuto por frontend. O recorder limita o DOM a 6.000 nós por snapshot e profundidade 80, faz flush periódico de 5 segundos e limita buffers em memória. DOM grande ou excedente pode não ter snapshot completo; nesses casos a tela não inventa um replay.

Retenção consultável de 24 horas, limitada adicionalmente a 15.000 eventos e 25 MiB/2.000 chunks de replay por servidor. O arquivo JSON é podado nas gravações subsequentes; não é uma política de eliminação física agendada em uma instalação sem atividade. Ao atingir os limites, registros antigos são removidos e segmentos podem perder o snapshot inicial. A interface só inicia playback quando esse snapshot existe.

O armazenamento central continua single writer. Não há fila durável, confirmação de entrega fim a fim ou garantia de capacidade para grandes populações de usuários. Origens permitidas são controles de navegador, não autenticação de um cliente público; dados são auto-reportados e um emissor fora do navegador pode falsificá-los. Exponha o coletor com TLS, limites no ingress e teste de carga adequado ao ambiente.

## Verificação executada

Testes unitários e APIs: autorização, origem, consentimento, mascaramento, persistência, deduplicação de vitals, Apdex, limpeza e rollback de gravação. Teste em Chromium real: injeção pelo proxy, fetch/XHR, erros, medições nativas LCP/TTFB, snapshot/mutações rrweb, dados protegidos ausentes no transporte, lista pública, revogação, DNT, consulta e playback no painel autenticado. O teste também verifica que a navegação SPA não adiciona uma amostra fictícia ao Apdex.

O cenário não valida todos os navegadores, todos os sites/CSP, um aplicativo mobile nativo nem carga de produção. Os comandos de teste são:

```sh
npm --prefix frontend run typecheck
npm --prefix frontend test
npm --prefix frontend run build
go test -race ./internal/... ./cmd/...
go vet ./internal/... ./cmd/...
# Com Playwright e Chromium instalados no ambiente de teste:
node scripts/smoke-rum.mjs /caminho/observa /caminho/vetra-injector
```

A descoberta de candidatos frontend pelo agente e a satisfação por sessão estão descritas em [WORKSPACE-OPERATIONS.md](WORKSPACE-OPERATIONS.md).
