export type Appearance = {
  theme: "dark" | "light";
  accent: "violet" | "cyan" | "green";
  density: "comfortable" | "compact";
};
export const defaultAppearance: Appearance = {
  theme: "dark",
  accent: "violet",
  density: "comfortable",
};
export function loadAppearance(): Appearance {
  try {
    const saved = JSON.parse(
      localStorage.getItem("observa-appearance-v1") || "{}",
    );
    return {
      theme: saved.theme === "light" ? "light" : "dark",
      accent: ["violet", "cyan", "green"].includes(saved.accent)
        ? saved.accent
        : "violet",
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
    title: "Criar dashboard",
    description: "Monte indicadores e gráficos dos seus serviços",
    path: "dashboards/novo",
    category: "Visualização",
  },
  {
    title: "Definir objetivo SLO",
    description: "Acompanhe disponibilidade ou latência",
    path: "slos/novo",
    category: "Confiabilidade",
  },
  {
    title: "Criar regra de alerta",
    description: "Detecte falhas e degradações de latência",
    path: "correlacao/novo",
    category: "Operação",
  },
  {
    title: "Configurar monitor",
    description: "Crie verificações HTTP e TCP",
    path: "templates/novo",
    category: "Monitoramento",
  },
  {
    title: "Conectar fonte de dados",
    description: "Configure consultas a fontes externas",
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
  { title: "Visão geral", path: "", description: "Seu centro de operação" },
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
