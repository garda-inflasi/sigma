"use client";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const SEMUA = "__semua";

/** Pilihan filter untuk bilah filter tabel; nilai kosong (undefined) = semua. */
export function FilterSelect({
  label,
  value,
  onChange,
  options,
  disabled,
}: {
  label: string;
  value: string | undefined;
  onChange: (value: string | undefined) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
}) {
  return (
    <Select
      value={value ?? SEMUA}
      onValueChange={(v) => onChange(v === SEMUA ? undefined : v)}
      disabled={disabled}
    >
      <SelectTrigger
        aria-label={label}
        className="h-10 min-w-48 rounded-md bg-white/70 dark:bg-white/5"
      >
        <SelectValue placeholder={label} />
      </SelectTrigger>
      <SelectContent className="rounded-xl">
        <SelectItem value={SEMUA}>{label}: semua</SelectItem>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
