import { Container } from "lucide-react";
import { Card, CardContent } from "../ui/card";

export function EmptyState() {
  return (
    <Card>
      <CardContent className="flex flex-col items-center justify-center py-12">
        <Container className="h-12 w-12 text-muted-foreground mb-4" />
        <h3 className="text-lg font-semibold mb-2">No containers running</h3>
        <p className="text-sm text-muted-foreground text-center max-w-md">
          No active containers found. Create a container using the API to see it
          appear here in real-time.
        </p>
      </CardContent>
    </Card>
  );
}
