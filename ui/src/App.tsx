import { BrowserRouter, Routes, Route } from "react-router-dom";
import { ThemeProvider } from "./providers/ThemeProvider";
import { QueryClientProvider } from "./providers/QueryClientProvider";
import { WebSocketProvider } from "./providers/WebSocketProvider";
import { Toaster } from "./components/ui/toaster";
import { ContainerList } from "./pages/ContainerList";
import { ContainerDetails } from "./pages/ContainerDetails";
import { ContainerLogs } from "./pages/ContainerLogs";
import igniteLogo from "./assets/ignite.png";

function App() {
  return (
    <BrowserRouter>
      <ThemeProvider defaultTheme="dark" storageKey="ignite-ui-theme">
        <QueryClientProvider>
          <WebSocketProvider>
            <div className="min-h-screen bg-background">
              <header className="border-b bg-card">
                <div className="container mx-auto px-4 py-4">
                  <div className="flex items-center gap-3">
                    <img src={igniteLogo} alt="Ignite Logo" className="w-8" />
                    <div>
                      <h1 className="text-2xl font-bold">Ignite</h1>
                      <p className="text-sm text-muted-foreground">
                        On-Demand Docker Container Management
                      </p>
                    </div>
                  </div>
                </div>
              </header>
              <main className="container mx-auto px-4 py-8">
                <Routes>
                  <Route path="/" element={<ContainerList />} />
                  <Route path="/containers/:id" element={<ContainerDetails />} />
                  <Route path="/containers/:id/logs" element={<ContainerLogs />} />
                </Routes>
              </main>
            </div>
            <Toaster />
          </WebSocketProvider>
        </QueryClientProvider>
      </ThemeProvider>
    </BrowserRouter>
  );
}

export default App;
