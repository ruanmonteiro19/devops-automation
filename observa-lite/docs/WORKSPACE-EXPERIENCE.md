# Atualização da experiência do workspace

## O que muda para quem usa

- Navegação por tarefas: workspace, aplicações, infraestrutura, exploração, confiabilidade e gestão. Áreas sem implementação foram retiradas do menu principal; os caminhos antigos permanecem compatíveis.
- Favoritos por navegador, grupos recolhíveis, navegação compacta acessível e central de comandos com Ctrl/Cmd+K.
- Criar abre os editores de dashboard, SLO, regra, monitor e fonte de dados. Nenhuma alteração é salva sem a ação de salvar do usuário.
- Visão geral com atalhos e primeiros passos, mantendo inventário e métricas derivados do servidor.
- Temas claro/escuro, destaques violeta/ciano/verde, tabelas confortáveis/compactas e exportação de preferências. Preferências e favoritos são locais ao navegador.
- Central de ajuda pesquisável, ajuda contextual, trecho de Collector copiável e diagnóstico agregado exportável sem tokens ou conteúdo de telemetria.
- Monitores HTTP/TCP: criar, editar, duplicar, pausar/retomar, filtrar, exportar e excluir com confirmação. Intervalos, timeout e status esperado editáveis.
- Dashboards: buscar, duplicar, importar JSON nativo para revisão, duplicar/reordenar painéis e definir meia largura/largura completa. Layout salvo no servidor. Importação não aceita formatos Grafana/Dynatrace.
- Fontes de dados: editar mantendo a identidade, criar sem exigir segredo opcional e exportar definição.
- Hosts/processos: buscar e exportar resultados; abrir detalhes do host com inventário e processos. Eventos de monitores: filtros e exportação.
- Acesso: trocar a credencial usada na aba e desconectar. Não cria tokens no servidor e não equivale a gerenciamento de contas ou RBAC.

## Atualizar a instalação existente

```bash
cd /opt/devops-automation
git switch feature/dynatrace-apm-investigation
git pull --ff-only origin feature/dynatrace-apm-investigation
cd observa-lite
docker compose up -d --build server agent
docker compose ps
curl -fsS http://127.0.0.1:8080/healthz
```

A configuração `.env` e o volume de dados existentes são mantidos. Recarregue a interface com Ctrl+F5 depois da atualização. Não use `down -v` para atualizar.

## Verificação e limites

56 testes de interface, TypeScript e build passaram, incluindo navegação recolhida/favoritos, edição de fonte, criação rápida, preferências, ajuda, monitores e dashboards. Backend validado com detector de concorrência e go vet; layout persistido e largura inválida rejeitada. Teste integrado executado com servidor local e UI compilada.

Não houve homologação visual em navegador real nesta sessão: o navegador remoto não alcança o servidor local, e o download do Chromium no executor não produziu um pacote utilizável. Os testes de interação usam jsdom. Docker não está disponível no executor; a execução Docker será verificada no ambiente de instalação. O build emite aviso pelo tamanho do bundle principal.

Essas mudanças melhoram a operação das funcionalidades existentes. Não adicionam OneAgent, Grail/DQL, SSO/RBAC, RUM, profiling ou RCA causal.

## Revisão visual: layout compacto

Referência: organização de telas na documentação oficial do SigNoz (https://signoz.io/docs/what-is-signoz/). Não implica paridade funcional com esse produto.

- Cantos de 4 px, superfícies neutras, tabelas compactas e seleção discreta.
- Remoção do banner promocional e dos títulos de apresentação. Rótulos e instruções usam frases diretas, com inicial maiúscula; valores recebidos da telemetria mantêm sua grafia original.
- Cores de ícones compartilhadas entre navegação desktop/mobile, favoritos, comandos e atalhos. Aplicações em ciano; infraestrutura em azul; alertas em laranja; dados em verde; traces e monitores em rosa; gestão e navegação geral em violeta.
- Temas claro/escuro e preferência de cor permanecem disponíveis.

Verificação: 56 testes de interface, TypeScript e build. A limitação de revisão visual em navegador descrita acima permanece.
