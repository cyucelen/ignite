import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft, Clock, Package, Terminal, Settings, Square, Trash2, FileText } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "../components/ui/alert-dialog";
import { useContainerStore } from "../store/containerStore";
import { formatDistanceToNow } from "date-fns";
import { api, APIClientError } from "../lib/api-client";
import { useToast } from "../hooks/use-toast";
import { useState } from "react";

export function ContainerDetails() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const getContainerById = useContainerStore((state) => state.getContainerById);
  const containerView = id ? getContainerById(id) : undefined;
  const container = containerView?.container;
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [showRemoveDialog, setShowRemoveDialog] = useState(false);

  // Stop container mutation
  const stopMutation = useMutation({
    mutationFn: () => {
      if (!id) throw new Error("Container ID is required");
      return api.containers.stop(id);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["containers"] });
      toast({
        title: "Container stopped",
        description: "The container has been stopped successfully.",
      });
    },
    onError: (error: Error) => {
      const errorMessage = error instanceof APIClientError
        ? error.message
        : "An unexpected error occurred";
      toast({
        title: "Failed to stop container",
        description: errorMessage,
        variant: "destructive",
      });
    },
  });

  // Delete container mutation
  const deleteMutation = useMutation({
    mutationFn: () => {
      if (!id) throw new Error("Container ID is required");
      return api.containers.delete(id);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["containers"] });
      toast({
        title: "Container removed",
        description: "The container has been removed successfully.",
      });
      // Navigate back to list after successful deletion
      navigate("/");
    },
    onError: (error: Error) => {
      const errorMessage = error instanceof APIClientError
        ? error.message
        : "An unexpected error occurred";
      toast({
        title: "Failed to remove container",
        description: errorMessage,
        variant: "destructive",
      });
    },
  });

  const handleStop = () => {
    stopMutation.mutate();
  };

  const handleRemove = () => {
    setShowRemoveDialog(true);
  };

  const confirmRemove = () => {
    setShowRemoveDialog(false);
    deleteMutation.mutate();
  };

  if (!container) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] space-y-4">
        <Package className="h-16 w-16 text-muted-foreground" />
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

  const getStateBadgeVariant = (
    state: string
  ): "default" | "secondary" | "destructive" | "outline" => {
    switch (state) {
      case "running":
        return "default";
      case "exited":
      case "stopped":
        return "secondary";
      case "creating":
        return "outline";
      default:
        return "secondary";
    }
  };

  return (
    <div className="space-y-6">
      {/* Header with Back Button */}
      <div className="flex items-center space-x-4">
        <Button variant="ghost" size="icon" onClick={() => navigate("/")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <h1 className="text-3xl font-bold">{container.image}</h1>
          <p className="text-sm text-muted-foreground">ID: {container.id}</p>
        </div>
        <Badge variant={getStateBadgeVariant(container.state)}>
          {container.state}
        </Badge>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate(`/containers/${id}/logs`)}
          >
            <FileText className="mr-2 h-4 w-4" />
            View Logs
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleStop}
            disabled={container.state !== "running" || stopMutation.isPending}
          >
            <Square className="mr-2 h-4 w-4" />
            {stopMutation.isPending ? "Stopping..." : "Stop"}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={handleRemove}
            disabled={deleteMutation.isPending}
          >
            <Trash2 className="mr-2 h-4 w-4" />
            {deleteMutation.isPending ? "Removing..." : "Remove"}
          </Button>
        </div>
      </div>

      {/* Container Info Grid */}
      <div className="grid gap-6 md:grid-cols-2">
        {/* Basic Information */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center">
              <Package className="mr-2 h-5 w-5" />
              Basic Information
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <dt className="text-sm font-medium text-muted-foreground">
                Image
              </dt>
              <dd className="text-sm mt-1">{container.image}</dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-muted-foreground">
                Docker ID
              </dt>
              <dd className="text-sm mt-1 font-mono break-all">
                {container.docker_id}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-muted-foreground">
                State
              </dt>
              <dd className="text-sm mt-1">
                <Badge variant={getStateBadgeVariant(container.state)}>
                  {container.state}
                </Badge>
              </dd>
            </div>
          </CardContent>
        </Card>

        {/* Timestamps */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center">
              <Clock className="mr-2 h-5 w-5" />
              Timestamps
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <dt className="text-sm font-medium text-muted-foreground">
                Created
              </dt>
              <dd className="text-sm mt-1">
                {formatDistanceToNow(new Date(container.created_at), {
                  addSuffix: true,
                })}
              </dd>
            </div>
            {container.started_at && (
              <div>
                <dt className="text-sm font-medium text-muted-foreground">
                  Started
                </dt>
                <dd className="text-sm mt-1">
                  {formatDistanceToNow(new Date(container.started_at), {
                    addSuffix: true,
                  })}
                </dd>
              </div>
            )}
            {container.exited_at && (
              <div>
                <dt className="text-sm font-medium text-muted-foreground">
                  Exited
                </dt>
                <dd className="text-sm mt-1">
                  {formatDistanceToNow(new Date(container.exited_at), {
                    addSuffix: true,
                  })}
                </dd>
              </div>
            )}
            {container.exit_code !== undefined && (
              <div>
                <dt className="text-sm font-medium text-muted-foreground">
                  Exit Code
                </dt>
                <dd className="text-sm mt-1">
                  <Badge
                    variant={container.exit_code === 0 ? "default" : "destructive"}
                  >
                    {container.exit_code}
                  </Badge>
                </dd>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Command */}
        {container.command && container.command.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Terminal className="mr-2 h-5 w-5" />
                Command
              </CardTitle>
            </CardHeader>
            <CardContent>
              <pre className="text-sm font-mono bg-muted p-4 rounded-md overflow-x-auto">
                {container.command.join(" ")}
              </pre>
            </CardContent>
          </Card>
        )}

        {/* Environment Variables */}
        {container.environment &&
          Object.keys(container.environment).length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <Settings className="mr-2 h-5 w-5" />
                  Environment Variables
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {Object.entries(container.environment).map(([key, value]) => (
                    <div
                      key={key}
                      className="flex items-start gap-2 text-sm bg-muted p-2 rounded"
                    >
                      <span className="font-medium text-muted-foreground min-w-[120px]">
                        {key}:
                      </span>
                      <span className="font-mono break-all">{value}</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}
      </div>

      {/* Last output - kept when the container exits with an error */}
      {container.last_logs && container.last_logs.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center">
              <FileText className="mr-2 h-5 w-5" />
              Last Output
              <Badge variant="destructive" className="ml-3">
                exit {container.exit_code}
              </Badge>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="text-sm font-mono bg-muted p-4 rounded-md overflow-x-auto whitespace-pre-wrap">
              {container.last_logs.join("\n")}
            </pre>
          </CardContent>
        </Card>
      )}

      {/* Resource Metrics - Only for running containers */}
      {container.state === "running" && (
        <Card>
          <CardHeader>
            <CardTitle>Resource Usage</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 md:grid-cols-3">
              <div className="space-y-2">
                <dt className="text-sm font-medium text-muted-foreground">
                  CPU Usage
                </dt>
                <dd className="text-2xl font-bold">N/A</dd>
                <p className="text-xs text-muted-foreground">
                  Real-time metrics coming soon
                </p>
              </div>
              <div className="space-y-2">
                <dt className="text-sm font-medium text-muted-foreground">
                  Memory Usage
                </dt>
                <dd className="text-2xl font-bold">N/A</dd>
                <p className="text-xs text-muted-foreground">
                  Real-time metrics coming soon
                </p>
              </div>
              <div className="space-y-2">
                <dt className="text-sm font-medium text-muted-foreground">
                  Network I/O
                </dt>
                <dd className="text-2xl font-bold">N/A</dd>
                <p className="text-xs text-muted-foreground">
                  Real-time metrics coming soon
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Remove Confirmation Dialog */}
      <AlertDialog open={showRemoveDialog} onOpenChange={setShowRemoveDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove Container</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to remove this container? This action cannot be undone.
              The container will be permanently deleted.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmRemove} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
