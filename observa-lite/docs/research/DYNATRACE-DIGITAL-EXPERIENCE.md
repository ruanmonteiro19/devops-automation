# Investigação — captura frontend e mobile

Data: 15/10/2026. Objetivo: compreender mecanismos públicos do Dynatrace e implementar uma solução própria na Vetra. Não houve acesso ao código-fonte do OneAgent, a binários mobile privados nem a um tenant autenticado. O playground retornou HTTP 401. Não se afirma engenharia reversa integral do produto.

## O que foi confirmado

| Área | Mecanismo documentado no Dynatrace | Consequência para a Vetra |
|---|---|---|
| Web com OneAgent | Em tecnologias suportadas, o agente em modo full stack modifica a resposta HTML e insere o JavaScript no head. Pode exigir reinício dos processos web. | Injeção sem editar arquivos é possível, mas requer controle do servidor ou do caminho de entrega. Nesta entrega a Vetra usa um proxy explícito, não um sensor OneAgent. |
| Web sem agente | Inclusão do JavaScript por tag ou mecanismo de implantação. O coletor recebe beacons do navegador. | SDK próprio servido pela Vetra, sem token administrativo no site monitorado. |
| Identidade | Sessões recebem identificadores; tags de usuário podem ser fornecidas por API. RUM Classic também documenta extração de metadados da página. | Sessões aleatórias e identificação opcional por ID opaco. A Vetra não extrai automaticamente nomes, emails, DOM de login ou variáveis de autenticação. |
| Replay web | Reconstrução por DOM, alterações e interações; não é uma captura de vídeo do monitor. | Uso de rrweb com redator próprio antes do transporte e sanitização adicional no servidor. |
| Privacidade | Opt-in, políticas de armazenamento, controles de rastreamento e mascaramento; regras de captura diferem das regras de playback. | Consentimento por padrão, replay opt-in separado, DNT/GPC, inputs sempre mascarados e textos bloqueados salvo lista pública explícita. |
| Android | Plugin Gradle e instrumentação de bytecode durante o build, além de SDK e APIs. | Não existe captura automática de uma interface nativa apenas por instalar o agente no backend. Um SDK e integração de build próprios ainda são necessários. |
| iOS | Adição do OneAgent ao projeto, configuração e novo build. | A Vetra ainda não fornece SDK iOS nativo. |
| Replay mobile novo | As páginas atuais descrevem replay Android/iOS além de replay em crashes e restrições por framework e versão. | Não assumir que uma antiga limitação de replay somente em crashes representa toda a versão nova. Nenhum replay nativo Vetra é anunciado. |
| Experience Vitals | Entrada para frontends web/mobile, sessões, erros, Web Vitals e métricas específicas de mobile. | Área exclusiva na Vetra para p75 de LCP, INP e CLS, mais FCP/TTFB. Sem SDK nativo, não há app start/crash/ANR nativos. |

## Fontes primárias

- [Injeção automática](https://docs.dynatrace.com/docs/observe/digital-experience/rum/web-frontends/initial-setup/configure-auto-injection)
- [RUM sem agente](https://docs.dynatrace.com/docs/observe/digital-experience/rum/web-frontends/initial-setup/set-up-agentless-monitoring)
- [Privacidade web](https://docs.dynatrace.com/docs/observe/digital-experience/rum/web-frontends/additional-configuration/data-privacy-web)
- [Identificação por metadados/API em RUM Classic](https://docs.dynatrace.com/docs/observe/digital-experience/rum-classic/web-applications/additional-configuration/identify-individual-users-for-session-analysis)
- [SDK JavaScript atual](https://docs.dynatrace.com/javascriptapi/doc-latest/documents/rum-javascript-sdk_Documentation.RUM_JavaScript_SDK_Overview.html)
- [Mascaramento no replay web](https://docs.dynatrace.com/docs/observe/digital-experience/session-replay/configure-session-replay-web)
- [Android: setup](https://docs.dynatrace.com/docs/observe/digital-experience/rum/mobile-frontends/android/id-01-initial-setup)
- [Plugin Gradle/bytecode](https://docs.dynatrace.com/docs/observe/digital-experience/mobile-applications/instrument-android-app/instrumentation-via-plugin)
- [iOS: setup](https://docs.dynatrace.com/docs/observe/digital-experience/rum/mobile-frontends/ios/id-01-initial-setup)
- [Replay Android atual](https://docs.dynatrace.com/docs/observe/digital-experience/session-replay-latest/configure-session-replay-mobile/latest-session-replay-android)
- [Replay iOS atual](https://docs.dynatrace.com/docs/observe/digital-experience/session-replay-latest/configure-session-replay-mobile/latest-session-replay-ios)
- [Experience Vitals](https://docs.dynatrace.com/docs/observe/digital-experience/rum/experience-vitals)
- [web-vitals](https://github.com/GoogleChrome/web-vitals)
- [rrweb](https://github.com/rrweb-io/rrweb/blob/main/guide.md)

## Uso do REA e evidência pública

Foi instalado e executado `rea-agents@6.5.0`. O HTML público de www.dynatrace.com referenciava um SDK público de RUM. Esse SDK foi baixado para um diretório temporário, sem executar beacons para o tenant do fornecedor e sem copiá-lo para o projeto.

- Origem: `https://js-cdn.dynatrace.com/jstag/145e12d594f/bf25977vwq/EA7C4B59F27D43EB_complete.js`
- Tamanho observado: 484.588 bytes.
- SHA-256 do arquivo: `aed9e9bf32f1d7dfb5c1e4cc472aa6c20da35cf4c60cded7d40ccc5db40071ac`.
- Comando: `npx --yes rea-agents@6.5.0 analyze-javascript-application /tmp/vetra-rea-dynatrace --max-semantic-nodes 2500 --max-semantic-nodes-per-file 2500 --analysis-timeout-ms 45000 --format json`.
- Evidência parcial: `ev_d2ac1c4476fbee41d3733b5a975baaab905f108a5020565cb291d4ae89f242d7`.
- Resultado: `capability_unavailable`, ao tentar preparar a inspeção de ownership do processo de análise. O procps retornou `fatal library error, lookup self`.

O REA autenticou o inventário do artefato, mas não concluiu a análise semântica nesse caminho. Isso não demonstra implementação interna, comportamento em execução nem cobertura funcional. Uma inspeção textual independente encontrou referências a APIs de identidade, envio de beacons, MutationObserver e PerformanceObserver; presença de nomes, isoladamente, não prova sua lógica ou configuração. As conclusões da tabela se apoiam nas fontes primárias, e a implementação Vetra foi validada independentemente.

A Vetra não incorpora código do SDK proprietário. O recorder/replayer utiliza rrweb e as medições usam web-vitals, com versões fixadas no lockfile. A investigação é de mecanismos e contratos observáveis, não uma alegação de paridade com Dynatrace.

## Observação REA concluída por CDP

Após a falha do worker estático, foi preparado um navegador Chromium de laboratório com a cópia temporária do SDK público. Todas as requisições fora da origem local foram bloqueadas antes de carregar o documento; nenhum beacon foi entregue ao fornecedor. O REA foi executado no mesmo processo de laboratório, anexado ao CDP já existente:

```sh
npx --yes rea-agents@6.5.0 observe-web-execution http://127.0.0.1:19382 TARGET_ID \
  --allowed-origins http://127.0.0.1:19381 --observation-ms 2000 --format json
```

A operação concluiu com coverage capturada. O resumo quantitativo está em [REA-RUNTIME-SUMMARY.json](REA-RUNTIME-SUMMARY.json). É um resumo extraído, não o bundle canônico do REA. Nenhum código do SDK foi publicado junto com ele. Essa observação confirma execução e posições de funções, mas não a semântica de cada função minificada ou sucesso de ingestão no tenant do fornecedor. As limitações da análise estática continuam válidas.
