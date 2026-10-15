# Organização, mapas e dados recentes

## O que está disponível

A navegação separa Painéis, Monitoramento, APM e experiência, Inventário, Coleta de dados, Análise e relatórios e Administração. Monitoramento oferece Problemas, Hosts, Equipamentos de rede, Dados recentes, Mapas, Gráficos e Eventos de monitores. Templates de rede têm página própria em Coleta de dados. As rotas anteriores continuam disponíveis.

O mapa de rede abre sem agrupamento e com portas recolhidas no equipamento. O botão Portas e o painel lateral expandem as interfaces. Selecionar um equipamento destaca sua vizinhança e reduz a opacidade das demais entidades; o seletor permite um, dois ou três saltos. A ligação equipamento–interface não consome um salto, portanto expandir portas não altera as aplicações relacionadas. A visão agrupada resume entidades; para drill-down individual, use Sem agrupamento.

As relações vêm de evidências distintas:

- LLDP anunciado pelo equipamento: vizinho, porta local e porta remota. Ative Descobrir vizinhos LLDP no cadastro e permita as OIDs na ACL SNMP.
- Socket observado pelo agente: relação lógica com o IP de equipamento cadastrado, sem afirmar a porta física do switch.
- OTLP: serviço ligado ao host por host.id ou host.name explícito, ou peer de span ligado ao IP exato de equipamento.

Os IPs de gerenciamento dos vizinhos LLDP recebem resolução DNS reversa (PTR) pelo DNS configurado no container. O nome DNS é priorizado para vizinhos ainda não cadastrados e pode identificar hosts/equipamentos por nome exato e único; entidades já cadastradas mantêm seu nome de inventário. O nome LLDP original fica preservado. Sem PTR, mantém-se LLDP/IP. São no máximo 64 IPs únicos por coleta, 500 ms por consulta e orçamento total de 3 segundos; falhas de DNS não interrompem a coleta. DNS identifica o vizinho observado, mas não descobre sozinho sua porta física.

Não criamos ligações por pertencer à mesma sub-rede. Nomes ambíguos e IPs repetidos não identificam automaticamente uma entidade. A porta LLDP local é associada ao ifIndex somente por nome/descrição exatos e únicos; seu número não é presumido igual ao índice IF-MIB. Vizinhos ainda não cadastrados aparecem como vizinhos anunciados. CDP, FDB/MAC/ARP, NetFlow e descoberta física universal não estão implementados.

O servidor reserva o limite de 500 nós visíveis para entidades principais e disponibiliza interfaces em interface_inventory, além dos nós que couberem. O frontend projeta arestas de interfaces recolhidas no equipamento. Há limite de 2000 arestas e limites de coleta documentados no guia SNMP; truncamento pode limitar a topologia.

## Dados recentes e gráficos

Dados recentes tem filtros de host, grupo, texto/chave, origem e estado, aplicação explícita dos filtros, atualização, ordenação, paginação, seleção e exportação CSV. Itens incompatíveis podem ser exibidos com seu motivo. Histórico abre as amostras realmente armazenadas, tabela e gráfico em janelas de 1, 6 ou 24 horas. A seção Gráficos oferece o catálogo dos itens numéricos com histórico.

Métricas SNMP usam o histórico nativo de métricas, sem banco Zabbix externo. A retenção atual é de 24 horas, limitada globalmente a 50000 pontos/16 MiB. A consulta de dados recentes limita a 5000 séries; não equivale a tendências históricas ilimitadas. Contadores inteiros grandes mantêm o texto exato; valores fora da precisão segura do JavaScript não são desenhados no gráfico. Inventário atual de processos e último resultado de monitores são identificados como valor atual, sem inventar histórico. Histogramas mostram resumo de contagem/soma.

## Templates modernos

Importação YAML, JSON e XML continua com prévia e relatório de compatibilidade. Além de itens escalares e LLD tradicional, o importador reconhece:

- Master SNMP_AGENT com walk de bases OID numéricas.
- Item DEPENDENT com SNMP_WALK_VALUE em formato numérico padrão, convertido em GET direto.
- Descoberta DEPENDENT com SNMP_WALK_TO_JSON em formato padrão, convertida em walks das colunas.
- Filtros MATCHES_REGEX/NOT_MATCHES_REGEX e combinações AND, OR e AND_OR, com expressões regulares compatíveis com Go RE2.
- Protótipos com macros descobertas validadas e preprocessamento numérico compatível.

Isso não executa o cache e o agendamento de dependências do Zabbix: frequência e número de consultas podem diferir. Expressões PCRE incompatíveis, filtros personalizados, JavaScript, JSONPath, agentes Zabbix, triggers, ações, templates herdados e demais componentes fora do subconjunto ficam reportados como incompatíveis ou com avisos. Não há paridade integral com o Zabbix. Reimporte templates já cadastrados para aplicar o novo compilador.

O template oficial FortiGate da branch release/7.0 passou de 36 itens compatíveis/101 incompatíveis para 111/17; triggers e gráficos possuem avisos separados. Esse resultado é análise e teste do importador, não certificação de um FortiGate físico.

## Investigação e validação

Fontes oficiais consultadas em 15/10/2026:

- [Estrutura dos menus Zabbix 7.0](https://github.com/zabbix/zabbix/blob/release/7.0/ui/include/classes/helpers/CMenuHelper.php).
- [Dados recentes](https://github.com/zabbix/zabbix/blob/release/7.0/ui/app/views/js/monitoring.latest.view.js.php).
- [Mapa e seleção de elementos](https://github.com/zabbix/zabbix/blob/release/7.0/ui/js/class.cmap.js).
- [Documentação de templates](https://www.zabbix.com/documentation/7.0/en/manual/xml_export_import/templates).
- [Template FortiGate](https://github.com/zabbix/zabbix/blob/release/7.0/templates/net/fortinet/fortigate_snmp/template_net_fortigate_snmp.yaml).

Os arquivos upstream foram usados para análise, sem incorporar seu código à distribuição. O REA 6.5 foi realmente invocado sobre class.cmap.js com analyze-javascript-application, timeout de 30 segundos e heap de 256 MiB. A execução foi bloqueada antes da análise pelo requisito process-ownership/prepare_owned_process: o ps do ambiente retorna fatal library error, lookup self. Evidência parcial: ev_24ddd778150c9f48983294b0a951b37b830b2a9692d0b2d14ad532bd9573f488. Não alegamos análise de runtime bem-sucedida pelo REA; a investigação utilizou leitura das fontes e documentação oficiais.

Validação executada:

- 91 testes de frontend; build de produção.
- Testes Go com detecção de corrida e go vet.
- UDP SNMPv1/v2c real, incluindo LLDP com número de porta diferente do ifIndex e template moderno compilado.
- Navegador com SNMP real de laboratório: cadastro, credenciais cifradas, importação, mapa recolhido, destaque LLDP/host/APM, expansão de portas, filtros, histórico e gráfico, página de templates e reinício do SQLite.
- SNMPv3 noAuth/auth/authPriv, SHA256/AES128 e SHA512/AES256 no laboratório.

A VM do cliente e equipamentos físicos não foram acessados nesta validação. Docker não está disponível neste ambiente; o servidor compilado foi executado diretamente. Consulte também [Rede nativa e templates](NETWORK-NATIVE-AND-ZABBIX.md).
