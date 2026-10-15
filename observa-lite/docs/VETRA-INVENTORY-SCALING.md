# Vetra Observability — inventário e capacidade

A marca apresentada na interface passa a ser Vetra Observability. Nomes de binários, variáveis OBSERVA_*, volumes, arquivos de estado, endpoints e serviços do Compose permanecem compatíveis com instalações existentes.

## Inventário

Em Hosts, selecione um host para editar o nome de exibição e o grupo de hosts. Digitar um grupo novo e salvar cria sua participação no inventário; as opções são derivadas dos grupos dos hosts existentes, sem cadastro separado de grupos vazios. O grupo aparece na tabela e pode ser encontrado pela busca. “Usar dados do agente” restaura nome e grupo coletados sem apagar nomes personalizados dos processos.

Em Processos, selecione um processo e edite o nome de exibição. Um nome vazio remove a personalização. A identidade usada para essa personalização é host + nome original + serviço; não depende de um PID duradouro. Processos iguais no mesmo serviço compartilham o nome personalizado. A operação verifica o PID e a identidade observados para rejeitar uma seleção desatualizada. Executáveis não são renomeados no sistema operacional.

Os ajustes são armazenados separadamente das observações do agente, com gravação atômica e reversão em memória em caso de falha. A rota PATCH /api/v1/hosts/{id} exige token administrativo. Novas coletas e reinicializações preservam as escolhas.

O servidor remove os processos de sistema conhecidos da construção dos mapas, inclusive conexões originadas por esses processos. A lista de processos mantém sua opção de diagnóstico para mostrar componentes do sistema, desativada por padrão. Inclui (sd-pam), systemd, bash, agetty e componentes conhecidos Linux/Windows. Aplicações desconhecidas continuam visíveis: não se deduz que um processo é descartável apenas por pouca memória ou pelo PID. O mapa de serviços APM continua representando os serviços declarados na telemetria OTLP.

O minimapa usa cores explícitas para nós, contornos, fundo e máscara. A tipografia da interface deixa de forçar caixa alta; identificadores coletados e abreviações técnicas preservam seu conteúdo.

## Gateways: o que escala hoje

O gateway é um relay HTTP sem armazenamento local de telemetria. É possível executar várias réplicas com certificados TLS válidos, registrar seus endpoints por Network Zone e usar o failover dos agentes. Cada réplica possui limite de concorrência configurável:

```sh
observa-gateway -max-inflight 64 -listen :18443 -upstream https://SEU-SERVIDOR -tls-cert /certs/fullchain.pem -tls-key /certs/privkey.pem
```

O limite padrão continua 32; são aceitos valores de 1 a 4096. Ao atingir o limite, o gateway responde 503 com Retry-After: 1. O cliente de descoberta pode tentar o próximo endpoint em erros transitórios. Validação e autenticação inválidas interrompem o envio. Nenhuma fila durável ou garantia de entrega é introduzida por essa mudança.

A descoberta ordena endpoints por prioridade: failover não equivale a balanceamento de carga. Para distribuir tráfego entre réplicas equivalentes, use um balanceador HTTPS ou configuração explícita dos emissores. Dimensione por testes de carga no ambiente: aumentar concorrência também aumenta consumo de memória e pressão sobre o servidor. /healthz verifica que o processo está atendendo, não a disponibilidade do upstream.

## Limite central ainda existente

O servidor central usa estado JSON local e sincronização dentro de um único processo. Não execute múltiplos servidores escrevendo o mesmo arquivo ou volume. Replicar gateways não torna esse backend distribuído nem fornece alta disponibilidade fim a fim.

Para escalar o produto horizontalmente com alta disponibilidade ainda é necessário substituir o armazenamento central por persistência compartilhada adequada para inventário e telemetria, introduzir ingestão com fila durável e consumidores independentes, coordenar tarefas periódicas e validar capacidade, recuperação e entrega sob falhas. Esta versão não promete esses recursos nem um volume de ingestão sem medição.

## Verificação

Testes cobrem persistência de nomes/grupos através de coleta e reinicialização, remoção de processos de sistema dos mapas, recuperação após falha de gravação, autorização administrativa, edição pela interface e sinalização de sobrecarga do relay. Build e checagem de tipos do frontend e testes Go com detector de corrida foram executados. Não foi feita validação visual do minimapa em navegador real neste ambiente nem teste de carga distribuída.
