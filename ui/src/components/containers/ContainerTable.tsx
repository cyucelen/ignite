import { useNavigate } from "react-router-dom";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../ui/table";
import { ContainerStateBadge } from "./ContainerStateBadge";
import { Skeleton } from "../ui/skeleton";
import type { UIContainerView } from "../../lib/types";
import { Activity, AlertCircle } from "lucide-react";

interface ContainerTableProps {
  containers: UIContainerView[];
  isLoading?: boolean;
}

export function ContainerTable({
  containers,
  isLoading,
}: ContainerTableProps) {
  const navigate = useNavigate();

  if (isLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-12 w-full" />
      </div>
    );
  }

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString();
  };

  const formatUptime = (createdAt: string, state: string) => {
    if (state !== "running") return "-";

    const now = new Date();
    const created = new Date(createdAt);
    const diff = now.getTime() - created.getTime();

    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days}d ${hours % 24}h`;
    if (hours > 0) return `${hours}h ${minutes % 60}m`;
    if (minutes > 0) return `${minutes}m`;
    return `${seconds}s`;
  };

  const formatMemory = (mb: number) => {
    if (mb >= 1024) {
      return `${(mb / 1024).toFixed(1)} GB`;
    }
    return `${mb} MB`;
  };

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>ID</TableHead>
            <TableHead>Image</TableHead>
            <TableHead>State</TableHead>
            <TableHead>CPU</TableHead>
            <TableHead>Memory</TableHead>
            <TableHead>Uptime</TableHead>
            <TableHead>Created</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {containers.map((view) => (
            <TableRow
              key={view.container.id}
              className="cursor-pointer hover:bg-muted/50 transition-colors"
              onClick={() => navigate(`/containers/${view.container.id}`)}
            >
              <TableCell className="font-mono text-xs">
                <div className="flex items-center gap-2">
                  {!view.is_healthy && (
                    <AlertCircle className="h-4 w-4 text-destructive" />
                  )}
                  {view.container.id.slice(0, 8)}
                </div>
              </TableCell>
              <TableCell>
                <div className="max-w-xs truncate" title={view.container.image}>
                  {view.container.image}
                </div>
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  <ContainerStateBadge state={view.container.state} />
                  {view.container.state === "running" && (
                    <Activity className="h-3 w-3 text-green-500 animate-pulse" />
                  )}
                </div>
              </TableCell>
              <TableCell>
                {view.metrics
                  ? `${view.metrics.cpu_percent.toFixed(1)}%`
                  : "-"}
              </TableCell>
              <TableCell>
                {view.metrics ? (
                  <div className="space-y-1">
                    <div>
                      {formatMemory(view.metrics.memory_usage_mb)}
                      {view.metrics.memory_limit_mb > 0 && (
                        <span className="text-muted-foreground">
                          {" "}
                          / {formatMemory(view.metrics.memory_limit_mb)}
                        </span>
                      )}
                    </div>
                    {view.metrics.memory_limit_mb > 0 && (
                      <div className="text-xs text-muted-foreground">
                        {view.metrics.memory_percent.toFixed(1)}%
                      </div>
                    )}
                  </div>
                ) : (
                  "-"
                )}
              </TableCell>
              <TableCell>
                {formatUptime(
                  view.container.created_at,
                  view.container.state
                )}
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {formatDate(view.container.created_at)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
