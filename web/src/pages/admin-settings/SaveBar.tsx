import { useState } from "react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { api } from "@/api/client";
import { AlertTriangle, RotateCcw } from "lucide-react";
import { toast } from "sonner";

interface SaveBarProps {
  dirtyCount: number;
  onSave: () => void;
  onDiscard: () => void;
  isSaving: boolean;
  restartRequired: boolean;
}

export function SaveBar({
  dirtyCount,
  onSave,
  onDiscard,
  isSaving,
  restartRequired,
}: SaveBarProps) {
  const [showRestartConfirm, setShowRestartConfirm] = useState(false);

  async function handleRestart() {
    try {
      await api("/admin/server/restart", { method: "POST" });
      toast.success("Server is restarting...");
    } catch {
      toast.error("Could not restart server. Please restart manually.");
    }
    setShowRestartConfirm(false);
  }

  return (
    <div className="surface-panel-subtle mt-6 rounded-xl p-4">
      {restartRequired && (
        <div className="text-foreground/80 mb-3 flex items-center gap-2 text-xs">
          <AlertTriangle className="h-3.5 w-3.5" />
          <span>Server restart required for changes to take effect.</span>
        </div>
      )}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span className="text-muted-foreground text-sm">
          {dirtyCount > 0 ? `${dirtyCount} unsaved change${dirtyCount > 1 ? "s" : ""}` : ""}
        </span>
        <div className="flex flex-col gap-2 sm:flex-row">
          {restartRequired && (
            <Button variant="outline" size="sm" onClick={() => setShowRestartConfirm(true)}>
              <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
              Restart Server
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={onDiscard} disabled={dirtyCount === 0}>
            Discard
          </Button>
          <Button size="sm" onClick={onSave} disabled={dirtyCount === 0 || isSaving}>
            {isSaving ? "Saving..." : "Save Changes"}
          </Button>
        </div>
      </div>
      <ConfirmDialog
        open={showRestartConfirm}
        onOpenChange={setShowRestartConfirm}
        title="Restart server?"
        description="The server will restart to apply configuration changes. Active streams will be interrupted."
        confirmLabel="Restart"
        variant="destructive"
        onConfirm={handleRestart}
      />
    </div>
  );
}
