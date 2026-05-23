import { useState } from "react";
import type { FormEvent } from "react";
import { api } from "@/api/client";
import type { CreateLibraryRequest, Library } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { LANGUAGES } from "@/player/utils/languageNames";
import { toast } from "sonner";
import { useWizardContext } from "../WizardContext";

export function LibraryStep() {
  const { markDone, refetchLibraries } = useWizardContext();
  const [libraryName, setLibraryName] = useState("Main Library");
  const [libraryPath, setLibraryPath] = useState("");
  const [libraryType, setLibraryType] = useState("movies");
  const [metadataLanguage, setMetadataLanguage] = useState("en");
  const [scanAfterCreate, setScanAfterCreate] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      const body: CreateLibraryRequest = {
        name: libraryName,
        type: libraryType,
        paths: [libraryPath],
        metadata_language: metadataLanguage,
      };
      const created = await api<Library>("/libraries", {
        method: "POST",
        body: JSON.stringify(body),
      });

      if (scanAfterCreate) {
        await api("/scan", {
          method: "POST",
          body: JSON.stringify({ library_id: created.id }),
        });
      }

      refetchLibraries();
      toast.success(scanAfterCreate ? "Library created and scan started" : "Library created");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create library");
    } finally {
      setSubmitting(false);
    }
  }

  function handleSkip() {
    markDone("library");
    toast.success("Library setup skipped");
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-3">
        <div className="space-y-1.5">
          <Label htmlFor="setup-library-name" className="text-xs">
            Name
          </Label>
          <Input
            id="setup-library-name"
            value={libraryName}
            onChange={(e) => setLibraryName(e.target.value)}
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="setup-library-type" className="text-xs">
            Type
          </Label>
          <Select value={libraryType} onValueChange={setLibraryType}>
            <SelectTrigger id="setup-library-type">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="movies">Movies</SelectItem>
              <SelectItem value="series">Series</SelectItem>
              <SelectItem value="mixed">Mixed</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="setup-metadata-lang">Metadata Language</Label>
          <Select value={metadataLanguage} onValueChange={setMetadataLanguage}>
            <SelectTrigger id="setup-metadata-lang">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LANGUAGES.map((lang) => (
                <SelectItem key={lang.code} value={lang.code}>
                  {lang.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="setup-library-path" className="text-xs">
          Path
        </Label>
        <Input
          id="setup-library-path"
          value={libraryPath}
          onChange={(e) => setLibraryPath(e.target.value)}
          placeholder="/media/movies"
          required
        />
      </div>
      <div className="flex items-center gap-2">
        <Switch
          id="setup-library-scan"
          checked={scanAfterCreate}
          onCheckedChange={setScanAfterCreate}
        />
        <Label htmlFor="setup-library-scan" className="text-xs">
          Scan after creating
        </Label>
      </div>
      <div className="flex gap-3 pt-3">
        <Button type="submit" disabled={submitting}>
          {submitting ? "Creating..." : "Create library"}
        </Button>
        <Button type="button" variant="ghost" onClick={handleSkip} disabled={submitting}>
          Skip
        </Button>
      </div>
    </form>
  );
}
