export type ObservaLanguage="pt-BR"|"en-US"|"es-ES";
export const validLanguage=(raw:string|null):ObservaLanguage=>
 raw==="en-US"||raw==="es-ES"?raw:"pt-BR";
type Dict=Record<string,string>;
const english:Dict={
 "Início":"Home","Visão geral":"Overview","Prioridades agora":"Priorities","Saúde do ambiente":"Environment health",
 "Aplicações / APM":"Applications / APM","Aplicações":"Applications","Serviços":"Services","Dependências":"Dependencies",
 "Traces":"Traces","Requisições":"Requests","Erros & exceções":"Errors & exceptions",
 "Infraestrutura":"Infrastructure","Hosts":"Hosts","Processos":"Processes","Containers":"Containers",
 "Dispositivos de rede":"Network devices","Interfaces":"Interfaces","Monitoramento":"Monitoring","Templates":"Templates",
 "Descobertas":"Discovery","Itens & métricas":"Items & metrics","Alertas":"Alerts","Incidentes":"Incidents",
 "Regras de correlação":"Correlation rules","Explorar":"Explore","Logs":"Logs","Métricas":"Metrics",
 "Consultas":"Queries","Visualização":"Visualization","Dashboards":"Dashboards","Editor de painéis":"Panel editor",
 "Modo TV":"TV mode","Automação":"Automation","IA & RCA":"AI & RCA","Integrações":"Integrations",
 "Extensões":"Extensions","Hub de Extensões":"Extensions Hub","Minhas extensões":"My extensions",
 "Administração":"Administration","Usuários & acesso":"Users & access","Autenticação (SSO/MFA)":"Authentication (SSO/MFA)",
 "Chaves de API":"API keys","Agentes":"Agents","Configurações":"Settings","Auditoria":"Audit","Ajuda":"Help",
 "Filtrar navegação":"Filter navigation","Pesquisar host, serviço, PID ou evento":"Search host, service, PID or event",
 "Gateways":"Gateways","Host Groups":"Host groups","Network Zones":"Network zones",
 "Produção":"Production","Homologação":"Staging"};
const spanish:Dict={
 "Início":"Inicio","Visão geral":"Visión general","Prioridades agora":"Prioridades","Saúde do ambiente":"Salud del entorno",
 "Aplicações / APM":"Aplicaciones / APM","Aplicações":"Aplicaciones","Serviços":"Servicios","Dependências":"Dependencias",
 "Requisições":"Solicitudes","Erros & exceções":"Errores y excepciones","Infraestrutura":"Infraestructura",
 "Processos":"Procesos","Dispositivos de rede":"Dispositivos de red","Monitoramento":"Monitorización",
 "Descobertas":"Descubrimientos","Itens & métricas":"Elementos y métricas","Alertas":"Alertas",
 "Incidentes":"Incidentes","Regras de correlação":"Reglas de correlación","Explorar":"Explorar",
 "Métricas":"Métricas","Consultas":"Consultas","Visualização":"Visualización","Editor de painéis":"Editor de paneles",
 "Modo TV":"Modo TV","Automação":"Automatización","IA & RCA":"IA y causa raíz",
 "Integrações":"Integraciones","Extensões":"Extensiones","Hub de Extensões":"Centro de extensiones",
 "Minhas extensões":"Mis extensiones","Administração":"Administración","Usuários & acesso":"Usuarios y acceso",
 "Autenticação (SSO/MFA)":"Autenticación (SSO/MFA)","Chaves de API":"Claves de API",
 "Agentes":"Agentes","Configurações":"Configuración","Auditoria":"Auditoría","Ajuda":"Ayuda",
 "Filtrar navegação":"Filtrar navegación","Pesquisar host, serviço, PID ou evento":"Buscar host, servicio, PID o evento",
 "Gateways":"Gateways","Host Groups":"Grupos de hosts","Network Zones":"Zonas de red",
 "Produção":"Producción","Homologação":"Homologación"};
export function navText(text:string,lang:ObservaLanguage):string{
 return lang==="en-US"?(english[text]??text):lang==="es-ES"?(spanish[text]??text):text;
}
