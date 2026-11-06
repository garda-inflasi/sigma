"use client";

import { X } from "lucide-react";
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
    <span className="inline-flex h-10 items-center gap-1.5 rounded-md bg-secondary px-3 text-sm">
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
      className="glass flex flex-wrap items-end gap-3 rounded-xl p-3"
      role="search"
      aria-label="Filter audit"
    >
      <label className="space-y-1 text-xs text-muted-foreground">
        Cari
        <Input
          type="search"
          value={filter.q ?? ""}
          onChange={(e) => set({ q: e.target.value })}
          placeholder="Aksi, objek, aktor, ID permintaan"
          className="h-10 w-60"
        />
      </label>

      <label className="space-y-1 text-xs text-muted-foreground">
        Aktor
        <Input
          value={filter.aktor ?? ""}
          onChange={(e) => set({ aktor: e.target.value })}
          placeholder="Nama, email, atau ID"
          className="h-10 w-52"
        />
      </label>

      <div className="space-y-1 text-xs text-muted-foreground">
        Aksi
        <Select
          value={filter.aksi ?? SEMUA}
          onValueChange={(v: string) =>
            set({ aksi: v === SEMUA ? undefined : v })
          }
        >
          <SelectTrigger className="h-10 min-w-48 rounded-md bg-white/70 dark:bg-white/5">
            <SelectValue />
          </SelectTrigger>
          <SelectContent className="rounded-xl">
            <SelectItem value={SEMUA}>Semua aksi</SelectItem>
            {AUDIT_AKSI.map((a) => (
              <SelectItem key={a} value={a}>
                {a}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-1 text-xs text-muted-foreground">
        Hasil
        <Select
          value={filter.hasil ?? SEMUA}
          onValueChange={(v: string) =>
            set({
              hasil: v === SEMUA ? undefined : (v as AuditFilter["hasil"]),
            })
          }
        >
          <SelectTrigger className="h-10 min-w-36 rounded-md bg-white/70 dark:bg-white/5">
            <SelectValue />
          </SelectTrigger>
          <SelectContent className="rounded-xl">
            <SelectItem value={SEMUA}>Semua hasil</SelectItem>
            {HASIL.map((h) => (
              <SelectItem key={h.value} value={h.value}>
                {h.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <label className="space-y-1 text-xs text-muted-foreground">
        Dari tanggal
        <Input
          type="date"
          value={filter.dari ?? ""}
          max={filter.sampai || undefined}
          onChange={(e) => set({ dari: e.target.value || undefined })}
          className="h-10"
        />
      </label>
      <label className="space-y-1 text-xs text-muted-foreground">
        Sampai tanggal
        <Input
          type="date"
          value={filter.sampai ?? ""}
          min={filter.dari || undefined}
          onChange={(e) => set({ sampai: e.target.value || undefined })}
          className="h-10"
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
        <button
          type="button"
          onClick={() => onChange({})}
          className="inline-flex h-10 items-center gap-1.5 rounded-md px-3 text-sm text-muted-foreground hover:bg-secondary hover:text-foreground"
        >
          <X className="size-4" /> Reset
        </button>
      )}
    </div>
  );
}
