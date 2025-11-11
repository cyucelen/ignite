import { useEffect, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useContainerStore } from "../store/containerStore";
import { api } from "../lib/api-client";
import { ContainerTable } from "../components/containers/ContainerTable";
import { EmptyState } from "../components/containers/EmptyState";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { Activity, Server, XCircle, Wifi, WifiOff } from "lucide-react";

export function ContainerList() {
  const containersMap = useContainerStore((state) => state.containers);
  const setContainers = useContainerStore((state) => state.setContainers);
  const isConnected = useContainerStore((state) => state.isConnected);

  // Memoize computed values to prevent infinite renders
  const containers = useMemo(() => {
    return Array.from(containersMap.values()).sort((a, b) => {
      if (a.container.state === "running" && b.container.state !== "running")
        return -1;
      if (a.container.state !== "running" && b.container.state === "running")
        return 1;
      return (
        new Date(b.container.created_at).getTime() -
        new Date(a.container.created_at).getTime()
      );
    });
  }, [containersMap]);

  const runningCount = useMemo(() => {
    return containers.filter((view) => view.container.state === "running")
      .length;
  }, [containers]);

  const stoppedCount = useMemo(() => {
    return containers.filter(
      (view) =>
        view.container.state === "exited" || view.container.state === "stopped"
    ).length;
  }, [containers]);

  // Fetch initial container list
  const { data, isLoading, error } = useQuery({
    queryKey: ["containers"],
    queryFn: async () => {
      const response = await api.containers.list();
      return response;
    },
    refetchInterval: 10000, // Refetch every 10 seconds as backup
  });

  // Update store when data arrives
  useEffect(() => {
    if (data?.containers) {
      setContainers(data.containers);
    }
  }, [data, setContainers]);

  return (
    <div className="space-y-6">
      {/* Header with stats */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Containers</h1>
          <p className="text-muted-foreground">
            Real-time container monitoring and management
          </p>
        </div>
        <div className="flex items-center gap-2">
          {isConnected ? (
            <Badge variant="default" className="gap-1">
              <Wifi className="h-3 w-3" />
              Connected
            </Badge>
          ) : (
            <Badge variant="destructive" className="gap-1">
              <WifiOff className="h-3 w-3" />
              Disconnected
            </Badge>
          )}
        </div>
      </div>

      {/* Stats cards */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Total Containers
            </CardTitle>
            <Server className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{containers.length}</div>
            <p className="text-xs text-muted-foreground">
              Active and stopped containers
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Running</CardTitle>
            <Activity className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {runningCount}
            </div>
            <p className="text-xs text-muted-foreground">
              Currently active containers
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Stopped</CardTitle>
            <XCircle className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stoppedCount}</div>
            <p className="text-xs text-muted-foreground">
              Exited or stopped containers
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Error state */}
      {error && (
        <Card className="border-destructive">
          <CardHeader>
            <CardTitle className="text-destructive">Error</CardTitle>
            <CardDescription>
              Failed to load containers: {error.message}
            </CardDescription>
          </CardHeader>
        </Card>
      )}

      {/* Container table or empty state */}
      {!error && (
        <>
          {containers.length === 0 && !isLoading ? (
            <EmptyState />
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>Container List</CardTitle>
                <CardDescription>
                  {isConnected
                    ? "Live updates enabled"
                    : "Updates paused - reconnecting..."}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <ContainerTable containers={containers} isLoading={isLoading} />
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  );
}
