export type Appearance = {
  theme: "dark" | "light";
  accent: "violet" | "cyan" | "green";
  density: "comfortable" | "compact";
};
export const defaultAppearance: Appearance = {
  theme: "dark",
  accent: "green",
  density: "comfortable",
};
export function loadAppearance(): Appearance {
  try {
    const saved = JSON.parse(
      localStorage.getItem("observa-appearance-v2") || "{}",
    );
    return {
      theme: saved.theme === "light" ? "light" : "dark",
      accent: "green",
      density: saved.density === "compact" ? "compact" : "comfortable",
    };
  } catch {
    return defaultAppearance;
  }
}
export function applyAppearance(value: Appearance) {
  document.documentElement.dataset["theme"] = value.theme;
  document.documentElement.dataset["accent"] = value.accent;
  document.documentElement.dataset["density"] = value.density;
}
export function downloadJSON(filename: string, data: unknown) {
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }),
  );
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}
export const quickActions = [
  {
    title: "Conectar cloud",
    description: "Inventário AWS, Azure ou GCP",
    path: "clouds/novo",
    category: "Integração",
  },
  {
    title: "Adicionar equipamento de rede",
    description: "Sistema e interfaces via SNMP Exporter",
    path: "dispositivos-rede/novo",
    category: "Integração",
  },

  {
    title: "Criar dashboard",
    description: "Indicadores e gráficos por serviço",
    path: "dashboards/novo",
    category: "Visualização",
  },
  {
    title: "Definir objetivo SLO",
    description: "Meta de disponibilidade ou latência",
    path: "slos/novo",
    category: "Confiabilidade",
  },
  {
    title: "Criar regra de alerta",
    description: "Limites de falhas e latência",
    path: "correlacao/novo",
    category: "Operação",
  },
  {
    title: "Configurar monitor",
    description: "Verificação HTTP ou TCP",
    path: "templates/novo",
    category: "Monitoramento",
  },
  {
    title: "Conectar fonte de dados",
    description: "Conexão com Prometheus, JSON ou Loki",
    path: "data-sources/novo",
    category: "Integração",
  },
  {
    title: "Enviar telemetria",
    description: "Guia de conexão OpenTelemetry",
    path: "ajuda/otlp",
    category: "Primeiros passos",
  },
];
export const workspaceDestinations = [
  {
    title: "Visões de topologia",
    path: "topologia",
    description: "Catálogo de mapas do ambiente",
  },
  {
    title: "Cloud providers",
    path: "clouds",
    description: "Configuração e inventário cloud",
  },
  {
    title: "Network devices",
    path: "dispositivos-rede",
    description: "Equipamentos e interfaces de rede",
  },

  { title: "Visão geral", path: "", description: "Hosts, serviços e eventos" },
  {
    title: "Serviços",
    path: "servicos",
    description: "Latência, tráfego e falhas",
  },
  {
    title: "Mapa de serviços",
    path: "service-mapping",
    description: "Dependências observadas",
  },
  {
    title: "Traces",
    path: "traces",
    description: "Investigue cada requisição",
  },
  { title: "Hosts", path: "hosts", description: "Inventário dos agentes" },
  {
    title: "Processos",
    path: "processos",
    description: "Processos e runtimes",
  },
  {
    title: "Problemas APM",
    path: "problemas-apm",
    description: "Problemas e evidências",
  },
  {
    title: "Dashboards",
    path: "dashboards",
    description: "Indicadores e painéis",
  },
  {
    title: "Logs",
    path: "telemetria-logs",
    description: "Eventos e correlação por trace",
  },
  { title: "Métricas", path: "metricas", description: "Sinais OTLP recebidos" },
  { title: "SLOs", path: "slos", description: "Objetivos e orçamento de erro" },
  {
    title: "Regras de alerta",
    path: "correlacao",
    description: "Gerencie limites e detecção",
  },
  {
    title: "Preferências",
    path: "configuracoes",
    description: "Tema, cor e densidade",
  },
  {
    title: "Central de ajuda",
    path: "ajuda",
    description: "Guias, dúvidas e diagnóstico",
  },
];

// Shared by desktop/mobile navigation, favorites, commands and shortcuts.
export function menuIconColor(path: string): string {
  const area = path.split("/")[0] || "home";
  if (
    [
      "servicos",
      "aplicacoes",
      "requisicoes",
      "service-mapping",
      "dependencias",
    ].includes(area)
  )
    return "menu-icon-cyan";
  if (
    [
      "problemas-apm",
      "erros",
      "incidentes",
      "alertas",
      "correlacao",
      "prioridades",
    ].includes(area)
  )
    return "menu-icon-orange";
  if (
    [
      "hosts",
      "processos",
      "process-mapping",
      "network-mapping",
      "agentes",
      "gateways",
      "host-groups",
      "network-zones",
    ].includes(area)
  )
    return "menu-icon-blue";
  if (
    [
      "telemetria-logs",
      "logs",
      "metricas",
      "data-sources",
      "itens-metricas",
    ].includes(area)
  )
    return "menu-icon-green";
  if (["traces", "templates", "extensoes", "minhas-extensoes"].includes(area))
    return "menu-icon-pink";
  return "menu-icon-violet";
}
