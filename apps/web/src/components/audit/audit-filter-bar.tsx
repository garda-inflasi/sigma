"use client";

import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DateField } from "@/components/ui/date-field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { AUDIT_AKSI, type AuditFilter } from "@/lib/api/audit";

const SEMUA = "__semua";
const HASIL = [
  { value: "BERHASIL", label: "Berhasil" },
  { value: "GAGAL", label: "Gagal" },
  { value: "DITOLAK", label: "Ditolak" },
] as const;

/** Filter terpasang dari panel detail (objek atau permintaan tertentu), bisa dilepas. */
function Terpasang({ label, onClear }: { label: string; onClear: () => void }) {
  return (
    <span className="inline-flex h-9 max-w-full items-center gap-1.5 rounded-md bg-secondary px-3 text-[13px]">
      <span className="num max-w-56 truncate">{label}</span>
      <button
        type="button"
        onClick={onClear}
        aria-label={`Lepas filter ${label}`}
        className="text-muted-foreground hover:text-foreground"
      >
        <X className="size-3.5" />
      </button>
    </span>
  );
}

/** Filter audit yang dijalankan server: pencarian, aktor, aksi, hasil, dan rentang tanggal. */
export function AuditFilterBar({
  filter,
  onChange,
}: {
  filter: AuditFilter;
  onChange: (filter: AuditFilter) => void;
}) {
  const set = (patch: Partial<AuditFilter>) =>
    onChange({ ...filter, ...patch });
  const aktif = Object.values(filter).some(Boolean);

  return (
    <div
      className="contents"
      role="search"
      aria-label="Filter audit"
    >
      <label className="w-full min-w-0 sm:w-36">
        <span className="sr-only">Cari</span>
        <Input
          type="search"
          value={filter.q ?? ""}
          onChange={(e) => set({ q: e.target.value })}
          placeholder="Aksi, objek, atau ID"
          className="h-8 md:text-[13px]"
        />
      </label>

      <label className="w-full min-w-0 sm:w-28">
        <span className="sr-only">Aktor</span>
        <Input
          value={filter.aktor ?? ""}
          onChange={(e) => set({ aktor: e.target.value })}
          placeholder="Aktor"
          title="Nama, email, atau ID aktor"
          className="h-8 md:text-[13px]"
        />
      </label>

      <Select
        value={filter.aksi ?? SEMUA}
        onValueChange={(v: string) =>
          set({ aksi: v === SEMUA ? undefined : v })
        }
      >
        <SelectTrigger
          size="sm"
          aria-label="Aksi"
          className="w-[calc(50%-0.2rem)] min-w-0 text-[13px] sm:w-auto sm:min-w-28 sm:max-w-36 md:text-[13px]"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={SEMUA}>Semua aksi</SelectItem>
          {AUDIT_AKSI.map((a) => (
            <SelectItem key={a} value={a}>
              {a}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        value={filter.hasil ?? SEMUA}
        onValueChange={(v: string) =>
          set({
            hasil: v === SEMUA ? undefined : (v as AuditFilter["hasil"]),
          })
        }
      >
        <SelectTrigger
          size="sm"
          aria-label="Hasil"
          className="w-[calc(50%-0.2rem)] min-w-0 text-[13px] sm:w-auto sm:min-w-24 sm:max-w-32 md:text-[13px]"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={SEMUA}>Semua hasil</SelectItem>
          {HASIL.map((h) => (
            <SelectItem key={h.value} value={h.value}>
              {h.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <label className="flex w-[calc(50%-0.2rem)] min-w-0 items-center gap-1 text-xs text-muted-foreground sm:w-auto">
        <span className="sr-only">Dari tanggal</span>
        <span aria-hidden>Dari</span>
        <DateField
          value={filter.dari ?? ""}
          max={filter.sampai || undefined}
          onValueChange={(v) => set({ dari: v || undefined })}
          className="w-full sm:w-[7.75rem]"
          kelasInput="h-8 text-foreground md:text-[13px]"
        />
      </label>
      <label className="flex w-[calc(50%-0.2rem)] min-w-0 items-center gap-1 text-xs text-muted-foreground sm:w-auto">
        <span className="sr-only">Sampai tanggal</span>
        <span aria-hidden>Sampai</span>
        <DateField
          value={filter.sampai ?? ""}
          min={filter.dari || undefined}
          onValueChange={(v) => set({ sampai: v || undefined })}
          className="w-full sm:w-[7.75rem]"
          kelasInput="h-8 text-foreground md:text-[13px]"
        />
      </label>

      {filter.entitas_id && (
        <Terpasang
          label={`Objek ${filter.entitas_id}`}
          onClear={() => set({ entitas_id: undefined })}
        />
      )}
      {filter.entitas && (
        <Terpasang
          label={`Entitas ${filter.entitas}`}
          onClear={() => set({ entitas: undefined })}
        />
      )}
      {filter.correlation_id && (
        <Terpasang
          label="Satu permintaan"
          onClear={() => set({ correlation_id: undefined })}
        />
      )}

      {aktif && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-8 px-2.5 text-[12.5px]"
          onClick={() => onChange({})}
        >
          <X className="size-3.5" /> Reset
        </Button>
      )}
    </div>
  );
}
