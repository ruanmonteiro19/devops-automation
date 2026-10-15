# Rede nativa e templates Zabbix

## Entrega

O Vetra Server consulta equipamentos diretamente por SNMPv1, SNMPv2c e SNMPv3. Novos cadastros não dependem de SNMP exporter. Perfis antigos de exporter continuam funcionando. Nenhum serviço adicional ou porta publicada no Docker é necessário: o servidor faz consultas UDP de saída para o equipamento.

A seleção de switch, roteador, firewall, wireless, UPS, impressora, storage, servidor/appliance ou sensor organiza o inventário. A disponibilidade real de métricas depende dos OIDs expostos pelo equipamento, das permissões SNMP e do template. Não há uma afirmação de compatibilidade integral com todos os fabricantes ou modelos.

O perfil padrão consulta SNMPv2-MIB, IF-MIB e IF-X-MIB: descrição, identificador, uptime, nome, localização, nomes/aliases das interfaces, estado administrativo/operacional, velocidade, contadores de entrada/saída, erros e descartes. Após duas coletas válidas, deriva bits/s e utilização. Reset de contador ou reboot detectado por uptime não gera taxa. Não infere wraps de contadores de 32 bits.

Credenciais SNMPv3: noAuthNoPriv, authNoPriv e authPriv; autenticação SHA/SHA256/SHA512; privacidade AES128 ou AES256 Blumenthal. Não implementa DES, MD5 nem AES256 Cisco/Reeder neste perfil.

## Cadastro

1. Em Dispositivos de rede, escolha Adicionar equipamento.
2. Na aba Geral, informe nome, tipo, grupo opcional, intervalo de coleta e template.
3. Na aba Conexão, informe IP/DNS, porta, versão e credenciais SNMP.
4. Salve e escolha Coletar agora. O estado e o inventário são atualizados de forma assíncrona.
5. Histórico de métricas abre o explorador nativo com o nome do equipamento e uptime preenchidos; outros nomes ficam disponíveis no filtro.

O coletor desta entrega é o Vetra Server. Coleta SNMP remota pelo Vetra Relay não está implementada; a interface não oferece essa opção. O equipamento precisa permitir o IP de origem do servidor ou do NAT da rede Docker na ACL SNMP.

Para redes internas, configure `OBSERVA_ALLOW_PRIVATE_TARGETS=true` no ambiente do servidor. Nomes DNS são resolvidos antes de conectar, os endereços são validados e a conexão usa o IP resolvido. O servidor não executa comandos SNMP SET.

As credenciais são cifradas com AES-GCM, nonce aleatório e associação ao ID do perfil. A chave é derivada do token administrativo existente. A API não devolve comunidade ou senhas: ao editar, mantenha as credenciais cadastradas ou use Alterar credenciais. Trocar o token administrativo exige recadastrar essas credenciais. Backups contêm o ciphertext; preserve o token separadamente. Use HTTPS para proteger o cadastro em trânsito.

## Importação de templates

Abra Templates de rede, selecione uma exportação Zabbix YAML, JSON ou XML e clique Analisar compatibilidade. A prévia não grava dados. Revise itens, descobertas e avisos; depois use Importar templates analisados. O template fica disponível no cadastro do equipamento. Templates vinculados a equipamentos não podem ser excluídos; o perfil nativo também é protegido.

| Recurso de template | Implementação |
| --- | --- |
| Item SNMP escalar com OID numérico | Executado |
| Sintaxe `get[OID]` com um OID | Executada |
| Macros de template que resultam em OID numérico | Expandidas, com limites |
| MULTIPLIER | Executado |
| SIMPLE_CHANGE / CHANGE_PER_SECOND | Executados com duas amostras, sem inventar taxa inicial/reset |
| DISCARD_UNCHANGED / HEARTBEAT | Valor coletado; histórico/heartbeat Zabbix não reproduzidos, aviso explícito |
| LLD `discovery[macro,OID,...]` | Walk e protótipos numéricos executados |
| LLD dependente SNMP_WALK_TO_JSON e filtros regulares | Executados no subconjunto documentado no guia de mapas |
| DEPENDENT com SNMP_WALK_VALUE e master `walk[...]` válido | Compilado para leitura SNMP direta; não reproduz cache master Zabbix |
| JSONPath, JavaScript e demais dependências | Não executados, reportados |
| Triggers, gráficos e templates vinculados | Não executados/expandidos, avisos explícitos |
| Intervalo por item, tendências e retenção Zabbix | Substituídos pelo intervalo/armazenamento do perfil Vetra, com aviso |
| Agente Zabbix, HTTP_AGENT, scripts, calculados, traps | Não executados |
| MIB simbólica ou macro não resolvida em OID | Item não executado, motivo visível |

Compatibilidade é parcial e analisada por componente. O relatório conta protótipos bloqueados por uma descoberta incompatível e mostra o motivo da descoberta. Importar um arquivo não significa reproduzir o motor completo do Zabbix. O perfil nativo continua coletando sistema/interfaces, mesmo quando um template acrescenta apenas alguns itens compatíveis.

Templates são dados; não executamos scripts embutidos. Aliases/anchors YAML, expansão excessiva de macros, profundidade excessiva e arquivos grandes são recusados. Valores de macros com nomes de comunidade, senha, segredo ou token não são importados.

## Armazenamento e limites

Resultados de inventário e segredos cifrados persistem no armazenamento nativo existente (JSON/SQLite/PostgreSQL). Métricas de rede são gravadas atomicamente junto ao resultado na mesma fila/histórico nativo de métricas OTLP. Prefixo: `network.snmp.`; atributos incluem ID/endereço do equipamento, grupo, template e índice da interface. O armazenamento mantém os limites globais de métricas e a retenção de 24 horas; isso não equivale às tendências de longo prazo do Zabbix.

- Até 50 perfis de integração; 4 tarefas concorrentes compartilhadas com outros coletores.
- Intervalo SNMP de 30 a 86400 segundos, padrão 300; timeout global de 90 segundos.
- Até 256 interfaces e 128 linhas por descoberta; 1000 leituras de itens/protótipos por coleta. Truncamento é sinalizado.
- Até 4096 pontos históricos de rede por resultado; limites globais de 50000 pontos/16 MiB continuam aplicados.
- Arquivo de template até 2 MiB, 50 templates por arquivo, catálogo até 100 templates/8 MiB.
- Até 2000 itens, 100 descobertas, 500 protótipos por descoberta e 200 macros por template.
- Inventário de recursos mantém os limites anteriores de 2 MiB por perfil e 16 MiB totais.

## Investigação e evidências

Referências oficiais consultadas em 2026-10-15:

- Zabbix: https://www.zabbix.com/documentation/7.0/en/manual/xml_export_import/templates
- Template oficial FortiGate: https://github.com/zabbix/zabbix/blob/release/7.0/templates/net/fortinet/fortigate_snmp/template_net_fortigate_snmp.yaml (blob `688e870980dc5784054d7ab273adb9fe8e07b0db`). O arquivo upstream foi usado como entrada de validação, sem ser incorporado à distribuição.
- NetBox: https://netbox.readthedocs.io/en/stable/introduction/ e https://github.com/netbox-community/netbox/blob/main/netbox/project-static/src/netbox.ts
- Gigamon: https://www.gigamon.com/content/dam/resource-library/english/feature-brief/fb-netflow-generation.pdf
- GoSNMP: https://github.com/gosnmp/gosnmp

NetBox é referência para inventário/IPAM e source of truth; não substitui o coletor de métricas. NetFlow/IPFIX descrevem fluxos de tráfego. Gigamon agrega visibilidade por pacotes/metadados e pode gerar fluxos. Esta entrega não implementa conector NetBox, receptor NetFlow/IPFIX/sFlow, inspeção de pacotes, traps ou descoberta CDP. A descoberta LLDP opcional foi adicionada nesta atualização. O mapa de processos/conexões IP existente não é um receptor NetFlow.

REA 6.5.0 foi invocado com `analyze-javascript-application` sobre o arquivo TypeScript selecionado do NetBox. A análise parou em `process-ownership/prepare_owned_process`: o `ps` do ambiente falhou com `fatal library error, lookup self`. Evidência parcial: `ev_8c8f3c049193973f64c746898a19e8b61c5382953924d19f0f3f416a64e755af`. Isso não comprova reconstrução do runtime NetBox, nem engenharia reversa de Gigamon. A implementação usa protocolos/bibliotecas abertas, documentação e leitura do código disponível.

Validação:

- Testes Go com agente UDP/BER próprio: SNMPv1/v2c, walk/bulkwalk, sistema/interfaces, scalar de template e LLD/protótipos.
- Importação YAML/JSON/XML, macros, scripts rejeitados, aliases recusados, multiplicador e taxas/reset.
- API e SQLite: prévia sem gravação, credenciais cifradas, edição que preserva segredo, chave errada recusada, reinício e remoção sem segredo órfão.
- Navegador + agente node-net-snmp: upload/prévia/importação, cadastro SNMP nativo sem exporter, coleta real, item adicional, script não executado, histórico, SNMPv3 em três níveis e SHA256/AES128 + SHA512/AES256, reinício do SQLite.
- O template oficial FortiGate foi analisado com 111 itens compatíveis e 17 componentes incompatíveis (inclui protótipos de descobertas bloqueadas); triggers/gráficos têm avisos adicionais. Não foi validado contra um FortiGate físico.

Scripts: `scripts/smoke-network.mjs` (dependência de laboratório node-net-snmp por `VETRA_SNMP_MODULE`) e `scripts/smoke-console.mjs` (regressão de contas/downloads e exporter legado). Docker não está disponível no ambiente de desenvolvimento para executar as imagens; compilação e execução do servidor e navegador foram verificadas diretamente.
