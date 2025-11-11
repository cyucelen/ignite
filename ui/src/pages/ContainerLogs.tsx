import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft, Download, RefreshCw, FileText } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { useContainerStore } from "../store/containerStore";
import { api, APIClientError } from "../lib/api-client";
import { useToast } from "../hooks/use-toast";
import { useState, useEffect, useRef } from "react";

export function ContainerLogs() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();
  const getContainerById = useContainerStore((state) => state.getContainerById);
  const containerView = id ? getContainerById(id) : undefined;
  const container = containerView?.container;
  const logsEndRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  // Fetch logs
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["container-logs", id],
    queryFn: () => {
      if (!id) throw new Error("Container ID is required");
      return api.containers.getLogs(id, { tail: 500, timestamps: true });
    },
    enabled: !!id,
    refetchInterval: 2000, // Refresh every 2 seconds for "live" logs
  });

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    if (autoScroll && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: "smooth" });
    }
  }, [data, autoScroll]);

  // Handle download logs
  const handleDownload = () => {
    if (!data?.logs || !container) return;

    const logsText = data.logs.join("\n");
    const blob = new Blob([logsText], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${container.id}-logs.txt`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);

    toast({
      title: "Logs downloaded",
      description: "Container logs have been saved to your downloads folder.",
    });
  };

  if (!container) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] space-y-4">
        <FileText className="h-16 w-16 text-muted-foreground" />
        <h2 className="text-2xl font-semibold">Container Not Found</h2>
        <p className="text-muted-foreground">
          The container you're looking for doesn't exist or has been removed.
        </p>
        <Button onClick={() => navigate("/")}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back to List
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <Button variant="ghost" size="icon" onClick={() => navigate(`/containers/${id}`)}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-3xl font-bold">Container Logs</h1>
            <p className="text-sm text-muted-foreground">
              {container.image} - ID: {container.id}
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setAutoScroll(!autoScroll)}
          >
            {autoScroll ? "Disable" : "Enable"} Auto-scroll
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isLoading}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${isLoading ? "animate-spin" : ""}`} />
            Refresh
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleDownload}
            disabled={!data?.logs || data.logs.length === 0}
          >
            <Download className="mr-2 h-4 w-4" />
            Download
          </Button>
        </div>
      </div>

      {/* Logs Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <span>Log Output</span>
            {data?.logs && (
              <span className="text-sm font-normal text-muted-foreground">
                {data.logs.length} lines
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading && (
            <div className="flex items-center justify-center py-8">
              <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
              <span className="ml-2 text-muted-foreground">Loading logs...</span>
            </div>
          )}

          {error && (
            <div className="py-8 text-center">
              <p className="text-destructive">
                {error instanceof APIClientError
                  ? error.message
                  : "Failed to load logs"}
              </p>
              <Button
                variant="outline"
                size="sm"
                className="mt-4"
                onClick={() => refetch()}
              >
                <RefreshCw className="mr-2 h-4 w-4" />
                Retry
              </Button>
            </div>
          )}

          {!isLoading && !error && data?.logs && (
            <>
              {data.logs.length === 0 ? (
                <div className="py-8 text-center text-muted-foreground">
                  <FileText className="h-12 w-12 mx-auto mb-4 opacity-50" />
                  <p>No logs available for this container.</p>
                </div>
              ) : (
                <div className="bg-black/90 rounded-md p-4 overflow-auto max-h-[600px] font-mono text-xs">
                  {data.logs.map((log, index) => (
                    <div
                      key={index}
                      className={`py-0.5 ${
                        log.toLowerCase().includes("error")
                          ? "text-red-400"
                          : log.toLowerCase().includes("warn")
                          ? "text-yellow-400"
                          : "text-green-400"
                      }`}
                    >
                      {log}
                    </div>
                  ))}
                  <div ref={logsEndRef} />
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
