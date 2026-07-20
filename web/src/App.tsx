import { Outlet } from "react-router-dom";
import { CommandPalette } from "./components/CommandPalette";
import { GlobalHotkeys } from "./components/GlobalHotkeys";
import { Sidebar } from "./components/Sidebar";
import { Tour } from "./components/Tour";
import { AuthProvider } from "./store/auth";
import { BrandingProvider } from "./store/branding";
import { ConfirmProvider } from "./store/confirm";
import { FavoritesProvider } from "./store/favorites";
import { PaletteProvider } from "./store/palette";
import { SidebarProvider, useSidebar } from "./store/sidebar";
import { TemplatesProvider } from "./store/templates";
import { ToastProvider } from "./store/toast";
import { TreeProvider } from "./store/tree";

// Общая оболочка: сайдбар слева, основная область справа (по макету).
function Shell() {
  const { open, setOpen } = useSidebar();
  return (
    <div className="flex h-[calc(100vh/var(--ui-zoom,1))] overflow-hidden bg-paper font-sans text-body">
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
              <FavoritesProvider>
                <TemplatesProvider>
                  <SidebarProvider>
                    <PaletteProvider>
                      <Shell />
                      <CommandPalette />
                      <GlobalHotkeys />
                      <Tour />
                    </PaletteProvider>
                  </SidebarProvider>
                </TemplatesProvider>
              </FavoritesProvider>
            </TreeProvider>
          </ConfirmProvider>
        </AuthProvider>
      </ToastProvider>
    </BrandingProvider>
  );
}
