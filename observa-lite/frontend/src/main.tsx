import { AuthGate } from "./components/user-access";
import React from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from "@tanstack/react-router";
import { Toaster } from "sonner";
import { ObservaApp } from "./components/observa-app";
import "@xyflow/react/dist/style.css";
import "./styles.css";
const rootRoute = createRootRoute({ component: () => <Outlet /> });
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: ObservaApp,
});
const allRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/$",
  component: ObservaApp,
});
const routeTree = rootRoute.addChildren([indexRoute, allRoute]);
const router = createRouter({ routeTree });
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
const client = new QueryClient({
  defaultOptions: { queries: { retry: false, staleTime: 5000 } },
});
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={client}>
      <AuthGate>
        <RouterProvider router={router} />
      </AuthGate>
      <Toaster richColors position="bottom-right" />
    </QueryClientProvider>
  </React.StrictMode>,
);
