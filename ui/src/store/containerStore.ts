import { create } from "zustand";
import type {
  Container,
  ContainerMetrics,
  UIContainerView,
} from "../lib/types";

interface ContainerStore {
  containers: Map<string, UIContainerView>;
  selectedContainerId: string | null;
  isConnected: boolean;

  getContainerList: () => UIContainerView[];
  getContainerById: (id: string) => UIContainerView | undefined;
  getSelectedContainer: () => UIContainerView | undefined;
  getRunningCount: () => number;
  getStoppedCount: () => number;

  setContainers: (containers: Container[]) => void;
  addContainer: (container: Container) => void;
  updateContainer: (id: string, updates: Partial<Container>) => void;
  removeContainer: (id: string) => void;
  updateMetrics: (id: string, metrics: ContainerMetrics) => void;
  setSelectedContainer: (id: string | null) => void;
  setConnected: (connected: boolean) => void;
  reset: () => void;
}

export const useContainerStore = create<ContainerStore>((set, get) => ({
  containers: new Map(),
  selectedContainerId: null,
  isConnected: false,

  getContainerList: () => {
    return Array.from(get().containers.values()).sort((a, b) => {
      if (a.container.state === "running" && b.container.state !== "running")
        return -1;
      if (a.container.state !== "running" && b.container.state === "running")
        return 1;
      return (
        new Date(b.container.created_at).getTime() -
        new Date(a.container.created_at).getTime()
      );
    });
  },

  getContainerById: (id: string) => {
    return get().containers.get(id);
  },

  getSelectedContainer: () => {
    const { selectedContainerId, containers } = get();
    if (!selectedContainerId) return undefined;
    return containers.get(selectedContainerId);
  },

  getRunningCount: () => {
    return Array.from(get().containers.values()).filter(
      (view) => view.container.state === "running"
    ).length;
  },

  getStoppedCount: () => {
    return Array.from(get().containers.values()).filter(
      (view) =>
        view.container.state === "exited" || view.container.state === "stopped"
    ).length;
  },

  setContainers: (containers: Container[]) => {
    set((state) => {
      const newMap = new Map(state.containers);
      containers.forEach((container) => {
        const existing = newMap.get(container.id);
        newMap.set(container.id, {
          container,
          metrics: existing?.metrics,
          is_healthy: existing?.is_healthy ?? true,
        });
      });
      return { containers: newMap };
    });
  },

  addContainer: (container: Container) => {
    set((state) => {
      const newMap = new Map(state.containers);
      newMap.set(container.id, {
        container,
        metrics: undefined,
        is_healthy: true,
      });
      return { containers: newMap };
    });
  },

  updateContainer: (id: string, updates: Partial<Container>) => {
    set((state) => {
      const existing = state.containers.get(id);
      if (!existing) return state;

      const newMap = new Map(state.containers);
      newMap.set(id, {
        ...existing,
        container: {
          ...existing.container,
          ...updates,
        },
      });
      return { containers: newMap };
    });
  },

  removeContainer: (id: string) => {
    set((state) => {
      const newMap = new Map(state.containers);
      newMap.delete(id);

      const selectedContainerId =
        state.selectedContainerId === id ? null : state.selectedContainerId;

      return { containers: newMap, selectedContainerId };
    });
  },

  updateMetrics: (id: string, metrics: ContainerMetrics) => {
    set((state) => {
      const existing = state.containers.get(id);
      if (!existing) return state;

      const is_healthy =
        metrics.cpu_percent <= 95 &&
        (metrics.memory_limit_mb === 0 || metrics.memory_percent <= 95);

      const newMap = new Map(state.containers);
      newMap.set(id, {
        ...existing,
        metrics,
        is_healthy,
      });
      return { containers: newMap };
    });
  },

  setSelectedContainer: (id: string | null) => {
    set({ selectedContainerId: id });
  },

  setConnected: (connected: boolean) => {
    set({ isConnected: connected });
  },

  reset: () => {
    set({
      containers: new Map(),
      selectedContainerId: null,
      isConnected: false,
    });
  },
}));
