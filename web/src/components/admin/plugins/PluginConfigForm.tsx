import { useEffect, useMemo, useState } from "react";

import type {
  ConnectionCheckResponse,
  PluginAdminFormField,
  PluginConfigSchema,
} from "@/api/types";
import { ConnectionCheckAction } from "@/components/admin/ConnectionCheckAction";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

type PluginConfigValue = Record<string, unknown>;

type Props = {
  schema: PluginConfigSchema;
  value?: PluginConfigValue;
  onSave: (key: string, value: PluginConfigValue) => void;
  onTest?: (key: string, value: PluginConfigValue) => Promise<ConnectionCheckResponse>;
  isSaving?: boolean;
  isTesting?: boolean;
};

type SupportedField = PluginAdminFormField & {
  inferredType?: "string" | "number" | "integer" | "boolean";
};

type ParsedObjectSchema = {
  supported: boolean;
  fields: SupportedField[];
};

function humanizeKey(value: string) {
  return value
    .split("_")
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function parseJSONSchema(schema: PluginConfigSchema): ParsedObjectSchema {
  try {
    const parsed = JSON.parse(schema.json_schema) as {
      type?: string;
      required?: string[];
      properties?: Record<string, { type?: string; title?: string; description?: string }>;
    };
    if (parsed.type !== "object" || !parsed.properties) {
      return { supported: false, fields: [] };
    }

    const fields = Object.entries(parsed.properties).map(([key, property]) => {
      const propertyType = property.type;
      if (!propertyType || !["string", "number", "integer", "boolean"].includes(propertyType)) {
        return null;
      }
      const control =
        propertyType === "boolean"
          ? "SWITCH"
          : propertyType === "number" || propertyType === "integer"
            ? "NUMBER"
            : "TEXT";
      return {
        key,
        label: property.title || humanizeKey(key),
        description: property.description,
        control,
        placeholder: "",
        required: parsed.required?.includes(key) ?? false,
        secret: false,
        multiline: false,
        options: [],
        rows: 0,
        inferredType: propertyType as "string" | "number" | "integer" | "boolean",
      } satisfies SupportedField;
    });

    if (fields.some((field) => field == null)) {
      return { supported: false, fields: [] };
    }
    return { supported: true, fields: fields.filter(Boolean) as SupportedField[] };
  } catch {
    return { supported: false, fields: [] };
  }
}

function defaultValueForField(field: SupportedField): string | boolean {
  if (field.default_value !== undefined) {
    if (typeof field.default_value === "boolean") {
      return field.default_value;
    }
    if (typeof field.default_value === "number") {
      return String(field.default_value);
    }
    if (typeof field.default_value === "string") {
      return field.default_value;
    }
  }
  if (field.control === "SWITCH") {
    return false;
  }
  return "";
}

function valueForField(field: SupportedField, configValue?: PluginConfigValue): string | boolean {
  const raw = configValue?.[field.key];
  if (typeof raw === "boolean") {
    return raw;
  }
  if (typeof raw === "number") {
    return String(raw);
  }
  if (typeof raw === "string") {
    return raw;
  }
  return defaultValueForField(field);
}

function buildPayload(fields: SupportedField[], draft: Record<string, string | boolean>) {
  const payload: PluginConfigValue = {};
  for (const field of fields) {
    const value = draft[field.key];
    if (field.control === "SWITCH") {
      payload[field.key] = Boolean(value);
      continue;
    }
    if (typeof value !== "string") {
      continue;
    }
    const trimmed = value.trim();
    if (trimmed === "") {
      continue;
    }
    if (
      field.control === "NUMBER" ||
      field.inferredType === "number" ||
      field.inferredType === "integer"
    ) {
      const numeric = Number(trimmed);
      if (!Number.isNaN(numeric)) {
        payload[field.key] = numeric;
        continue;
      }
    }
    payload[field.key] = value;
  }
  return payload;
}

export function PluginConfigForm({
  schema,
  value,
  onSave,
  onTest,
  isSaving = false,
  isTesting = false,
}: Props) {
  const parsedFallback = useMemo(() => parseJSONSchema(schema), [schema]);
  const fields = useMemo<SupportedField[]>(() => {
    if (schema.admin_form?.fields?.length) {
      return schema.admin_form.fields;
    }
    return parsedFallback.fields;
  }, [parsedFallback.fields, schema.admin_form?.fields]);

  const supported =
    fields.length > 0 && (schema.admin_form?.fields?.length ? true : parsedFallback.supported);

  const [draft, setDraft] = useState<Record<string, string | boolean>>(() =>
    Object.fromEntries(fields.map((field) => [field.key, valueForField(field, value)])),
  );
  const [testResult, setTestResult] = useState<ConnectionCheckResponse | null>(null);

  useEffect(() => {
    setDraft(Object.fromEntries(fields.map((field) => [field.key, valueForField(field, value)])));
  }, [fields, value]);

  function setField(key: string, nextValue: string | boolean) {
    setTestResult(null);
    setDraft((current) => ({ ...current, [key]: nextValue }));
  }

  async function handleTest() {
    if (!onTest) {
      return;
    }

    try {
      setTestResult(await onTest(schema.key, buildPayload(fields, draft)));
    } catch (error) {
      setTestResult({
        success: false,
        message: error instanceof Error ? error.message : "Connection check failed.",
      });
    }
  }

  if (!supported) {
    return (
      <div className="space-y-2 rounded-md border border-amber-500/30 bg-amber-500/5 p-3">
        <Label>{schema.title || schema.key}</Label>
        <p className="text-muted-foreground text-sm">
          This plugin uses a configuration schema shape that the admin form does not support yet.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3 rounded-md border p-3">
      <div className="space-y-1">
        <Label>{schema.title || schema.key}</Label>
        {schema.description ? (
          <p className="text-muted-foreground text-xs">{schema.description}</p>
        ) : null}
      </div>

      <div className="grid gap-4">
        {fields.map((field) => (
          <div key={field.key} className="space-y-2">
            <div className="space-y-1">
              <Label htmlFor={`${schema.key}-${field.key}`}>
                {field.label || humanizeKey(field.key)}
              </Label>
              {field.description ? (
                <p className="text-muted-foreground text-xs">{field.description}</p>
              ) : null}
            </div>

            {field.control === "SWITCH" ? (
              <div className="flex items-center gap-3 rounded-md border px-3 py-2">
                <Switch
                  checked={Boolean(draft[field.key])}
                  onCheckedChange={(checked) => setField(field.key, checked)}
                />
                <span className="text-sm">{field.label || humanizeKey(field.key)}</span>
              </div>
            ) : field.control === "SELECT" ? (
              <Select
                value={String(draft[field.key] ?? "")}
                onValueChange={(nextValue) => setField(field.key, nextValue)}
              >
                <SelectTrigger id={`${schema.key}-${field.key}`}>
                  <SelectValue placeholder={field.placeholder || "Select"} />
                </SelectTrigger>
                <SelectContent>
                  {(field.options ?? []).map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : field.control === "TEXTAREA" || field.multiline ? (
              <textarea
                id={`${schema.key}-${field.key}`}
                className="border-border bg-background min-h-24 w-full rounded-md border px-3 py-2 text-sm"
                rows={field.rows && field.rows > 0 ? field.rows : 4}
                value={String(draft[field.key] ?? "")}
                placeholder={field.placeholder}
                onChange={(event) => setField(field.key, event.target.value)}
              />
            ) : (
              <Input
                id={`${schema.key}-${field.key}`}
                type={
                  field.control === "PASSWORD" || field.secret
                    ? "password"
                    : field.control === "NUMBER"
                      ? "number"
                      : "text"
                }
                value={String(draft[field.key] ?? "")}
                placeholder={field.placeholder}
                onChange={(event) => setField(field.key, event.target.value)}
              />
            )}
          </div>
        ))}
      </div>

      <div className="flex flex-wrap items-center gap-3">
        {onTest ? (
          <ConnectionCheckAction
            onClick={handleTest}
            result={testResult}
            isPending={isTesting}
            disabled={isSaving}
          />
        ) : null}
        <Button
          size="sm"
          variant="outline"
          disabled={isSaving || isTesting}
          onClick={() => onSave(schema.key, buildPayload(fields, draft))}
        >
          {schema.admin_form?.submit_label || "Save config"}
        </Button>
      </div>
    </div>
  );
}
