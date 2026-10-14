# Loki opcional — logs com retenção de 7 dias

Esta instalação NÃO é obrigatória para iniciar o Observa Lite. O instalador Linux não implanta Loki nem coleta logs automaticamente. A consulta do Observa Lite funciona somente após conectar um Loki existente ou ativar este serviço em um ambiente autorizado.

## Subir Loki para LAB
No diretório observa-lite/deploy/loki, execute:

    docker compose up -d

O Loki fica ligado SOMENTE ao endereço 127.0.0.1:3100. Para consultar com o backend local Observa Lite, o administrador deve habilitar explicitamente OBSERVA_ALLOW_PRIVATE_TARGETS=true no serviço Go e reiniciar o serviço. Esse ajuste permite que os demais Data Sources e templates alcancem IPs privados: ative somente em ambiente confiável; o ideal é restringir destinos por allowlist em uma evolução futura.

No menu Data Sources crie:
- Nome: Loki local
- Tipo: loki
- URL: http://127.0.0.1:3100

No menu Logs use um seletor LogQL como {service="api"} e clique em Consultar.

## Como os logs chegam ao Loki?
O Observa Lite não captura, armazena ou indexa mensagens de log nesta versão. Conecte um agente compatível (ex.: Grafana Alloy, OpenTelemetry Collector com exportação adequada ou outro shipper Loki); configure somente os arquivos e serviços autorizados. Os logs não aparecem até que algum shipper os envie.

## Limites intencionais
- Retenção por idade: 168 horas (7 dias) aplicada pelo compactor, com exclusão assíncrona. Não é limite absoluto de tamanho em GB.
- Indexação: TSDB v13 somente de labels; conteúdo comprimido em chunks.
- Limites: taxa de ingestão 2 MB/s com pico de 4 MB, linha máxima 128 KiB; leituras pela plataforma até 200 linhas, período de até 24 horas e resposta limitada.
- Cache em memória até 20 MB para resultados.
- Evite labels de alta cardinalidade (trace_id, PID, request_id, hash, caminho único). Prefira labels estáveis como ambiente, servico, host e nivel.
- Monitore capacidade de disco, cardinalidade, compressão, erros de ingestão e memória. O Loki não apaga logs automaticamente por porcentagem de uso do disco.
- Nunca ingira dados sensíveis sem política de mascaramento e autorização.

## Produção
Essa configuração é SOMENTE de laboratório e NÃO possui autenticação nem replicação. Em produção adote TLS, autenticação, isolamento de tenants, armazenamento de objetos durável (S3 compatível, GCS, etc.), backup, HA conforme volume e política de retenção. Não exponha diretamente a porta 3100.

Referências: https://grafana.com/docs/loki/latest/configure/storage/ e https://grafana.com/docs/loki/latest/operations/storage/retention/
