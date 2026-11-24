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
        size="sm"
        aria-label={label}
        className="w-full min-w-0 text-[13px] sm:w-auto sm:min-w-28 sm:max-w-44 md:text-[13px]"
      >
        <SelectValue placeholder={label} />
      </SelectTrigger>
      <SelectContent>
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
