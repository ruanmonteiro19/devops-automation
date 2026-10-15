import type { LucideIcon } from "lucide-react";
import {
  Activity,
  Bot,
  Boxes,
  Cloud,
  Database,
  Gauge,
  GitBranch,
  Globe,
  Network,
  Radio,
  Server,
  Webhook,
} from "lucide-react";

export type ExtensionCategory =
  | "APM"
  | "Cloud"
  | "Banco de dados"
  | "Rede"
  | "Integrações"
  | "Observabilidade"
  | "IA";
export type ExtensionManifest = {
  id: string;
  name: string;
  description: string;
  author: string;
  category: ExtensionCategory;
  type: string;
  version: string;
  compatibility: string;
  status: "Catálogo" | "Planejado";
  icon: LucideIcon;
};

const catalog: Array<[string, ExtensionCategory, LucideIcon, string]> = [
  [
    "Prometheus",
    "Observabilidade",
    Gauge,
    "Consulte séries temporais e indicadores PromQL.",
  ],
  [
    "Zabbix",
    "Observabilidade",
    Activity,
    "Integre eventos e inventário monitorado.",
  ],
  [
    "Grafana",
    "Observabilidade",
    Gauge,
    "Referencie dashboards e painéis existentes.",
  ],
  [
    "OpenTelemetry",
    "APM",
    Radio,
    "Receba traces OTLP HTTP com atributos e eventos.",
  ],
  [
    "SNMP",
    "Rede",
    Network,
    "Interfaces e sistema via SNMP Exporter configurado.",
  ],
  [
    "PostgreSQL",
    "Banco de dados",
    Database,
    "Saúde, sessões e consultas PostgreSQL.",
  ],
  [
    "Oracle",
    "Banco de dados",
    Database,
    "Telemetria operacional para Oracle Database.",
  ],
  [
    "MySQL",
    "Banco de dados",
    Database,
    "Métricas e disponibilidade de instâncias MySQL.",
  ],
  [
    "SQL Server",
    "Banco de dados",
    Database,
    "Indicadores operacionais do Microsoft SQL Server.",
  ],
  [
    "Elasticsearch",
    "Observabilidade",
    Database,
    "Saúde de clusters, shards e índices.",
  ],
  ["Loki", "Observabilidade", Server, "Pesquisa e correlação de logs no Loki."],
  ["Tempo", "APM", GitBranch, "Consulta de traces distribuídos do Tempo."],
  [
    "Kubernetes",
    "Cloud",
    Boxes,
    "Inventário e relações de workloads Kubernetes.",
  ],
  [
    "Jenkins",
    "Integrações",
    GitBranch,
    "Relacione execuções de CI/CD a eventos.",
  ],
  [
    "Dynatrace API",
    "Integrações",
    Activity,
    "Importe contexto autorizado via API externa.",
  ],
  ["AWS", "Cloud", Cloud, "Inventário EC2 por região."],
  ["GCP", "Cloud", Cloud, "Inventário de VMs Compute Engine."],
  ["Azure", "Cloud", Cloud, "Contexto operacional do Microsoft Azure."],
  ["OCI", "Cloud", Cloud, "Inventário e métricas Oracle Cloud."],
  ["HTTP", "Integrações", Globe, "Monitores HTTP configuráveis."],
  [
    "Webhook",
    "Integrações",
    Webhook,
    "Receba ou encaminhe eventos por webhook.",
  ],
  [
    "MCP",
    "IA",
    Bot,
    "Exponha contexto operacional via Model Context Protocol.",
  ],
];

export const extensionManifest: ExtensionManifest[] = catalog.map(
  ([name, category, icon, description], index) => ({
    id: name.toLowerCase().replaceAll(" ", "-"),
    name,
    category,
    icon,
    description,
    author: "Observa Lite",
    type: category === "Integrações" ? "Conector" : "Extensão",
    version: index < 5 ? "Prevista" : "Roteiro",
    compatibility: "Observa Lite 1.x+",
    status: index < 4 ? "Catálogo" : "Planejado",
  }),
);
