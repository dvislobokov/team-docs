import { Outlet } from "react-router-dom";
import { CommandPalette } from "./components/CommandPalette";
import { Sidebar } from "./components/Sidebar";
import { Tour } from "./components/Tour";
import { AuthProvider } from "./store/auth";
import { BrandingProvider } from "./store/branding";
import { ConfirmProvider } from "./store/confirm";
import { PaletteProvider } from "./store/palette";
import { SidebarProvider, useSidebar } from "./store/sidebar";
import { ToastProvider } from "./store/toast";
import { TreeProvider } from "./store/tree";

// Общая оболочка: сайдбар слева, основная область справа (по макету).
function Shell() {
  const { open, setOpen } = useSidebar();
  return (
    <div className="flex h-screen overflow-hidden bg-paper font-sans text-body">
      {/* затемнение под выехавшим сайдбаром на мобильном */}
      {open && (
        <div
          className="fixed inset-0 z-30 bg-ink/30 backdrop-blur-[1px] md:hidden"
          onClick={() => setOpen(false)}
        />
      )}
      <Sidebar />
      <main id="main-scroll" className="scroll flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  );
}

export function App() {
  return (
    <BrandingProvider>
      <ToastProvider>
        <AuthProvider>
          <ConfirmProvider>
            <TreeProvider>
              <SidebarProvider>
                <PaletteProvider>
                  <Shell />
                  <CommandPalette />
                  <Tour />
                </PaletteProvider>
              </SidebarProvider>
            </TreeProvider>
          </ConfirmProvider>
        </AuthProvider>
      </ToastProvider>
    </BrandingProvider>
  );
}
