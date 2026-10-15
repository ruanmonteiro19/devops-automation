# Integrações Cloud e Network Devices

## Escopo entregue

Conectores somente leitura, executados pelo servidor:

| Conector | Consulta | Dados |
|---|---|---|
| AWS | EC2 DescribeInstances, Signature V4 | Instâncias de uma região, estado, zona, tipo, IPs, VPC/subnet e nome da tag Name |
| Azure | OAuth client credentials + ARM Resources List | Inventário da subscription, ID, tipo, localização e nome; não representa saúde de runtime |
| GCP | Compute instances.aggregatedList, OAuth bearer | VMs do projeto, zona, tipo, IP e estado |
| SNMP Exporter | HTTP /snmp, módulo/auth configuráveis | IF-MIB (interfaces, estados, velocidade, contadores, erros/descartes) e métricas de sistema/CPU/memória/temperatura publicadas por módulos compatíveis |

Não é monitoramento completo de todos os serviços de cloud: sem CloudWatch/Azure Monitor/Cloud Monitoring, custos, IAM inventory, descoberta automática de todas as contas/regiões ou mudanças nos recursos. GCP usa access token fornecido no ambiente; tokens temporários GCP/AWS precisam ser renovados pelo administrador e reinjetados no container. Azure solicita token novo a cada coleta.

Não existe driver SNMP nativo no Observa: SNMP v2/v3 fica a cargo do Prometheus SNMP Exporter. LLDP/CDP e topologia física não são coletados. Indicadores de fabricante dependem dos MIBs/módulos do exporter; não é possível garantir “todas as informações” de qualquer equipamento.

## Credenciais no Docker

O serviço server carrega `.env` por env_file. A interface salva apenas nomes de variáveis, nunca credenciais. Edite o `.env` local (não envie seu conteúdo ao chat):

```dotenv
# Prefixo do perfil: OBSERVA_AWS_PROD
OBSERVA_AWS_PROD_ACCESS_KEY_ID='SEU_ACCESS_KEY'
OBSERVA_AWS_PROD_SECRET_ACCESS_KEY='SEU_SECRET'
# Apenas se usar sessão temporária:
# OBSERVA_AWS_PROD_SESSION_TOKEN='SEU_SESSION_TOKEN'

# Prefixo: OBSERVA_AZURE_PROD
OBSERVA_AZURE_PROD_TENANT_ID='SEU_TENANT'
OBSERVA_AZURE_PROD_CLIENT_ID='SEU_CLIENT_ID'
OBSERVA_AZURE_PROD_CLIENT_SECRET='SEU_CLIENT_SECRET'

# Prefixo: OBSERVA_GCP_PROD
OBSERVA_GCP_PROD_ACCESS_TOKEN='SEU_OAUTH_ACCESS_TOKEN'
```

Use aspas simples em valores literais para preservar caracteres como `$`. Permissões mínimas: AWS `ec2:DescribeInstances` na região; Azure Reader na subscription; GCP `compute.instances.list` (ex.: Compute Viewer). Use somente o bloco dos provedores que for conectar.

Aplique as variáveis:

```bash
chmod 600 .env
docker compose up -d --build --force-recreate server agent
```

Na interface, abra **Cloud Providers → Conectar Cloud**, selecione provedor, região/subscription/project e prefixo correspondente. Salve e escolha **Coletar Agora**. A coleta é assíncrona; acompanhe o estado e o inventário na tela. Uma linha de inventário não é atestado de saúde do recurso.

## Network Devices

Se já possui SNMP Exporter acessível ao servidor, use-o. Para uma instância no mesmo Compose:

```bash
# Como root, na pasta observa-lite:
sh scripts/setup-snmp-exporter.sh
```

O script baixa o snmp.yml oficial fixado em v0.30.1 e preserva qualquer arquivo existente. Edite `deploy/snmp/snmp.yml`: configure seu perfil auth (community ou SNMPv3) e os módulos do equipamento. O arquivo usa UID/GID65534 e permissão600; não o versione nem o compartilhe. A configuração upstream contém perfis de exemplo: ajuste-os para o seu ambiente.

```bash
# Após editar a configuração:
chown 65534:65534 deploy/snmp/snmp.yml
chmod 600 deploy/snmp/snmp.yml
# Para acesso a equipamentos e exporter em endereços privados:
sed -i 's/^OBSERVA_ALLOW_PRIVATE_TARGETS=.*/OBSERVA_ALLOW_PRIVATE_TARGETS=true/' .env
docker compose --profile network up -d snmp-exporter
docker compose up -d --force-recreate server agent
```

O exporter não publica porta no host; o server o acessa pela rede Compose. Em **Network Devices → Adicionar Equipamento**:

- URL base: `http://snmp-exporter:9116`.
- Target: IP literal do equipamento (IPv4/IPv6); nomes DNS não são aceitos nesta versão.
- Module: `if_mib` como ponto de partida; acrescente módulos existentes na sua configuração para outros indicadores (ex.: system, fabricante).
- Auth: nome do seu perfil auth, sem community/senha no formulário.

Garanta conectividade do exporter ao equipamento na porta/protocolo SNMP configurados e permissão de leitura nos OIDs necessários. Depois de alterar snmp.yml, reinicie somente o exporter: `docker compose --profile network restart snmp-exporter`.

Após duas coletas bem-sucedidas, `observa_in/out_bits_per_second` e `observa_in/out_utilization_percent` são derivados dos contadores e da velocidade recebida. Primeira coleta, intervalo inválido, reinício detectado pelo uptime ou queda do contador não geram taxas. A ausência de uptime limita a detecção de reinícios. Preferência HC64; fallback32 não tenta inferir wraps. Precisão é a da exposição numérica Prometheus; não há histórico temporal completo de interfaces nesta versão.

## Persistência e limites

- Coleta automática a cada5min, até4tarefas compartilhadas com os monitores; timeout90s por coleta.
- Até50perfis, 10páginas e1000recursos por perfil; resultados truncados sinalizados.
- Resposta HTTP limitada8MiB, payload de recursos2MiB por perfil e16MiB de recursos no inventário total.
- Equipamentos: até256interfaces,64métricas originais por interface (mais os derivados de tráfego) e200métricas de sistema; truncamento sinalizado.
- Falha mantém o último inventário, com estado de erro, timestamp da tentativa e timestamp da última coleta válida. Recursos desatualizados aparecem como stale no mapa.
- Alterar a conexão/escopo invalida o inventário anterior; pausar preserva o último inventário. Resultados de configurações antigas em andamento são rejeitados.
- Endpoints de cloud fixos e sem redirects; acesso privado ao exporter/target exige a opção explícita de destinos privados.

## Visões de topologia

`/topologia` oferece visões unificada, infraestrutura, problemas, serviços, Kubernetes, AWS, Azure, GCP, AI e network devices. Agrupamento por tipo ou host/localização; expansão/recolhimento, seleção de nós, exportação e layouts radial/horizontal/vertical/grade.

Kubernetes usa apenas `k8s.cluster.name`, `k8s.namespace.name` e `k8s.pod.uid/name` realmente recebidos nos recursos OTLP: não consulta a API do cluster. AI usa atributos gen_ai explícitos de spans. Problemas representam regras/serviços, não RCA causal. Contenção de inventário não é tráfego de rede. A visão unificada combina essas fontes conhecidas, sem correlação automática de hosts locais com VMs de cloud.

API: `GET/POST /api/v1/integrations`, `DELETE /api/v1/integrations/{id}`, `POST /api/v1/integrations/{id}/collect` (202), `GET /api/v1/topology?view=all|infrastructure|services|problems|kubernetes|aws|azure|gcp|ai|devices`. Todas exigem token administrativo. O canvas limita200nós; API500nós/2000relações, com indicador de truncamento.

## Fontes e validação

- AWS: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html e https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_sigv-create-signed-request.html
- Azure: https://learn.microsoft.com/en-us/rest/api/resources/resources/list?view=rest-resources-2021-04-01 e https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow
- GCP: https://docs.cloud.google.com/compute/docs/instances/get-list
- Exporter: https://github.com/prometheus/snmp_exporter e https://github.com/prometheus/snmp_exporter/releases

Validação cobre contratos HTTP/paginação/autenticação com transportes simulados, exporter HTTP local real, persistência/reinício, estados de falha, limites e componentes de interface. Sem credenciais de contas/equipamentos, não houve homologação contra cloud real ou equipamento SNMP real. Docker e revisão visual em navegador continuam limitados pelo ambiente desta sessão. O vídeo enviado orientou o catálogo, agrupamentos e ações dos mapas; não há equivalência integral ao Smartscape on Grail.

Verificação final desta entrega: 67 testes de interface; testes Go com detector de concorrência e go vet; smoke-apm.py e smoke-integrations.py com executável local compilado. O teste de integrações percorre exporter HTTP → coleta assíncrona → taxas → topologia → retenção após falha → reinício → exclusão.
