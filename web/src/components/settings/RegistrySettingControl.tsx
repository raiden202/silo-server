import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { SettingDefinition } from "@/lib/settingsManifest";

const EMPTY_SELECT_VALUE = "__empty__";

interface RegistrySettingControlProps {
  definition: SettingDefinition;
  value: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}

export function RegistrySettingControl({
  definition,
  value,
  disabled = false,
  onChange,
}: RegistrySettingControlProps) {
  if (definition.control === "switch") {
    return (
      <Switch
        checked={value === "true"}
        disabled={disabled}
        onCheckedChange={(checked) => onChange(checked ? "true" : "false")}
      />
    );
  }

  if (definition.control === "slider") {
    const numericValue = Number(value || definition.defaultValue || 0);
    return (
      <div className="flex w-full max-w-[260px] items-center gap-3">
        <Slider
          value={[numericValue]}
          min={definition.min}
          max={definition.max}
          step={definition.step}
          disabled={disabled}
          onValueCommit={(values) => onChange(String(values[0] ?? numericValue))}
        />
        <span className="text-muted-foreground min-w-16 text-right text-xs font-medium">
          {numericValue}
          {definition.unit ? ` ${definition.unit}` : ""}
        </span>
      </div>
    );
  }

  return (
    <Select
      value={value === "" ? EMPTY_SELECT_VALUE : value}
      onValueChange={(nextValue) => onChange(nextValue === EMPTY_SELECT_VALUE ? "" : nextValue)}
      disabled={disabled}
    >
      <SelectTrigger className="w-full min-w-[180px] sm:w-[220px]">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {(definition.options ?? []).map((option) => (
          <SelectItem
            key={option.value || EMPTY_SELECT_VALUE}
            value={option.value === "" ? EMPTY_SELECT_VALUE : option.value}
          >
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
