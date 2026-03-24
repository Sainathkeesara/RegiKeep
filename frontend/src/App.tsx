import { Toaster } from "@/components/ui/toaster";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { AppSidebar } from "@/components/AppSidebar";
import { ThemeProvider } from "@/hooks/use-theme";
import { ThemeToggle } from "@/components/ThemeToggle";
import Dashboard from "./pages/Dashboard";
import Groups from "./pages/Groups";
import Audit from "./pages/Audit";
import ArchivePage from "./pages/ArchivePage";
import Export from "./pages/Export";
import SettingsPage from "./pages/SettingsPage";
import NotFound from "./pages/NotFound";

const queryClient = new QueryClient();

const App = () => (
  <QueryClientProvider client={queryClient}>
    <ThemeProvider>
      <TooltipProvider>
        <Toaster />
        <Sonner />
        <BrowserRouter>
          <SidebarProvider>
            <div className="min-h-screen flex w-full">
              <AppSidebar />
              <div className="flex-1 flex flex-col min-w-0">
                <header className="h-10 flex items-center justify-between border-b border-border px-2 shrink-0">
                  <SidebarTrigger />
                  <ThemeToggle />
                </header>
              <main className="flex-1 p-4 overflow-auto">
                <Routes>
                  <Route path="/" element={<Dashboard />} />
                  <Route path="/groups" element={<Groups />} />
                  <Route path="/audit" element={<Audit />} />
                  <Route path="/archive" element={<ArchivePage />} />
                  <Route path="/export" element={<Export />} />
                  <Route path="/settings" element={<SettingsPage />} />
                  <Route path="*" element={<NotFound />} />
                </Routes>
              </main>
            </div>
          </div>
          </SidebarProvider>
        </BrowserRouter>
      </TooltipProvider>
    </ThemeProvider>
  </QueryClientProvider>
);

export default App;
