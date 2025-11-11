import { Badge } from "../ui/badge";
import type { ContainerState } from "../../lib/types";

interface ContainerStateBadgeProps {
  state: ContainerState;
}

export function ContainerStateBadge({ state }: ContainerStateBadgeProps) {
  const getVariant = (state: ContainerState) => {
    switch (state) {
      case "running":
        return "default"; // Green
      case "creating":
        return "secondary"; // Gray
      case "exited":
      case "stopped":
        return "destructive"; // Red
      case "removed":
        return "outline"; // Light gray
      default:
        return "secondary";
    }
  };

  return (
    <Badge variant={getVariant(state)} className="capitalize">
      {state}
    </Badge>
  );
}
