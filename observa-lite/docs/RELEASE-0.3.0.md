# Entrega Observa Lite APM 0.3.0-lab

## Executar por Docker Compose

Na pasta do projeto:

```bash
./scripts/docker-init.sh
docker compose up -d --build server agent
```

Acesse http://127.0.0.1:8080. O token administrativo está no `.env` gerado localmente; o script não exibe nem substitui segredos existentes. A descoberta do host em container é para Linux. O agente passivo não produz automaticamente spans: configure aplicações/Collector para exportar os sinais conforme [OTLP](APM-OTLP.md).

Esta entrega não atualizou imagens GHCR, não alterou a VM e não publicou uma branch remota. Para usar o código novo, reconstrua a partir desta pasta; uma imagem antiga `qa-black-green` não contém essas mudanças.

## Executável Linux incluído no pacote

O arquivo `dist/observa` é o servidor/agente Linux amd64 com a interface incorporada; `dist/observa-gateway` é o relay TLS. Para o servidor local:

```bash
./scripts/docker-init.sh
set -a
. ./.env
set +a
./dist/observa server --listen 127.0.0.1:8080
```

Nenhuma instalação atual é substituída. O estado fica em `observa-data/state.json` nesta pasta. Guarde seu `.env` fora de versionamento. O pacote não inclui tokens nem dados do ambiente.

## Executar de fonte

Requer Go1.23+ e Node22+ (ou as imagens do Dockerfile):

```bash
cd frontend
npm ci --legacy-peer-deps
npm run build
cd ..
mkdir -p internal/server/public/app
cp -R frontend/dist/. internal/server/public/app/
mkdir -p dist
go build -o dist/observa ./cmd/observa
./scripts/docker-init.sh
set -a
. ./.env
set +a
./dist/observa server
```

O binário de servidor deve incorporar a UI React compilada. Sem essa cópia, a compilação simples Go pode servir o HTML histórico de fallback.

## Caminho de uso

1. Envie traces/logs/métricas do seu Collector para os endpoints OTLP HTTP autenticados.
2. Abra Serviços: selecione serviço, período, endpoint e métricas RED.
3. Investigue falhas, banco, chamadas externas, mensageria, instâncias e versões.
4. Abra um trace; selecione spans e veja atributos/eventos e logs correlacionados.
5. Crie SLOs para objetivos de disponibilidade ou duração.
6. Crie Regras APM para limites de erros ou P95 e acompanhe Problemas APM.
7. Crie dashboards com indicadores e séries por serviço e período.

## Verificação reproduzível

```bash
go test -race ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
cd frontend
npm run typecheck
npm run test
npm run build
cd ..
python3 scripts/smoke-apm.py --binary ./dist/observa
```

O script sobe um servidor local isolado com credenciais temporárias, envia três sinais OTLP, verifica análise/SLO/problema/dashboard e reinicia o processo para comprovar persistência. Não toca dados do seu ambiente.

Backend passou com race detector; interface passou43 testes em8 arquivos, typecheck e build. O fluxo HTTP real passou. O navegador remoto bloqueia localhost: validação visual integrada em navegador não foi concluída. Há aviso de bundle frontend grande; o build passa.

## Escopo e limites

A cobertura completa está em [FEATURE-COVERAGE](FEATURE-COVERAGE.md). O armazenamento ainda é JSON limitado, com retenção lógica24h; não suporta escala, HA ou isolamento de múltiplos clientes como uma plataforma empresarial. Não há autoinstrumentação/profiling equivalente ao OneAgent, Grail/DQL, RCA causal, RUM, replay, SSO/RBAC ou workflows executáveis. Integrações de infraestrutura anteriores permanecem com o escopo já documentado.

O REA6.5.0 está configurado, mas sua análise ficou bloqueada por erro de inspeção de processos do ambiente. OmniRoute3.8.51 está instalado sem provedor configurado; não foi usado para inferência. Não alegamos acesso ao código interno proprietário do Dynatrace.
