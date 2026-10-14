# Observa Lite em Docker (Linux amd64 / arm64)

**MVP de laboratório:** Servidor Go e frontend Lovable/React (um único container), agente de descoberta Linux separado, armazenamento persistente e Loki opcional. Sem SSO/MFA, eBPF, logs coletados automaticamente nem bancos TSDB internos.

## Requisitos
Linux com Docker Engine e plugin Compose v2; permissões para rodar containers com PID/network namespace do host. Docker Desktop Windows/macOS não permite a mesma inspeção de processos do host.

## Instalar e iniciar

    cd observa-lite
    ./scripts/docker-init.sh
    docker compose up -d --build server agent
    docker compose ps
    docker compose logs -f --tail=50 server agent

O instalador gera .env com dois tokens aleatórios, sem exibi-los no terminal. Para ler a chave do painel:

    sed -n 's/^OBSERVA_ADMIN_TOKEN=//p' .env

A interface é http://127.0.0.1:8080 no servidor Linux. Acesse remotamente via túnel SSH:

    ssh -L 8080:127.0.0.1:8080 usuario@servidor

A primeira descoberta pode levar ~30 segundos. A API /healthz não requer token; /api/v1/snapshot requer chave.

## Imagens GHCR (opcional)
Se o workflow Docker tiver publicado as imagens (o pacote pode ser privado no GHCR):

- ghcr.io/ruanmonteiro19/observa-lite-server:edge
- ghcr.io/ruanmonteiro19/observa-lite-agent:edge

Defina OBSERVA_SERVER_IMAGE e OBSERVA_AGENT_IMAGE no .env, use docker login ghcr.io se necessário, e execute:

    docker compose pull server agent
    docker compose up -d --no-build server agent

Sem acesso ao GHCR, mantenha o build local com --build.

## Loki opcional com retenção
    docker compose --profile logs up -d loki

No Observa Lite crie Data Source tipo loki com URL **http://loki:3100**, na rede interna do Compose. Para permitir destinos IP privados, mude OBSERVA_ALLOW_PRIVATE_TARGETS=true em .env e reinicie só o backend com docker compose up -d --force-recreate server. Essa opção libera outras fontes/templates para destinos privados: **use apenas em laboratório isolado**. Loki ainda precisa receber mensagens por algum coletor externo como Grafana Alloy. Nada é coletado automaticamente pelo agente Observa Lite. A configuração Loki existente limita retenção a 7 dias; não há quota fixa em GB.

## Segurança e isolamento
Servidor: imagem distroless como UID 65532, rootfs somente leitura, volume persistente, sem capabilities e com portas restritas ao loopback por padrão.
Agente: root, network/pid/UTS do host e capabilities SYS_PTRACE/DAC_READ_SEARCH para identificar sockets de outros processos. Esse acesso é sensível; use somente em servidores próprios/autorizados. Não depende de Docker socket nem de privileged:true. SELinux/AppArmor ou hidepid podem reduzir a visibilidade.
Loki: serviço sob perfil logs; porta de diagnóstico restrita ao loopback, sem autenticação no exemplo de laboratório. Não exponha externamente.

## Persistência e limites
Volume observa-data armazena o JSON da versão atual (não é SQLite). Volume loki-data armazena chunks e índices do Loki. Limites de RAM: 384 MiB servidor, 256 MiB agente, 768 MiB Loki opcional. Limites de CPU: 1 / 0,5 / 1. Esses limites podem precisar de ajustes em ambientes maiores.

    docker compose down         # Para os containers e conserva os volumes
    docker compose down -v      # APAGA dados e logs. NÃO USE sem backup.

Para atualizar mantendo volumes:

    docker compose up -d --build server agent

Para diagnosticar:
    docker compose ps
    docker compose logs --tail=100 server agent
    curl -fsS http://127.0.0.1:8080/healthz

Consulte deploy/loki/README.md e deploy/storage/ARCHITECTURE.md para limitações e próximos passos.
