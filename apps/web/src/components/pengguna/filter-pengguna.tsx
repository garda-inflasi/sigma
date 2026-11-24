"use client";

import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  jumlahFilterAktif,
  STATUS_PENGGUNA,
  type KunciFilter,
  type StateTabelPengguna,
} from "@/lib/pengguna/tabel";
import { LAPIS_META, ROLES, ROLE_IDS, type Lapis } from "@/lib/rbac/roles";
import { cn } from "@/lib/utils";

const SEMUA = "__semua";

interface Kontrol {
  kunci: KunciFilter;
  label: string;
  options: { value: string; label: string }[];
}

const LABEL_STATUS: Record<(typeof STATUS_PENGGUNA)[number], string> = {
  AKTIF: "Aktif",
  NONAKTIF: "Nonaktif",
  MENUNGGU: "Menunggu aktivasi",
};

const KONTROL: Kontrol[] = [
  {
    kunci: "peran",
    label: "Peran",
    options: ROLE_IDS.map((r) => ({ value: r, label: ROLES[r].label })),
  },
  {
    kunci: "lapis",
    label: "Lapis",
    options: (Object.keys(LAPIS_META) as Lapis[]).map((l) => ({
      value: l,
      label: LAPIS_META[l].label.split(" — ")[0],
    })),
  },
  {
    kunci: "status",
    label: "Status",
    options: STATUS_PENGGUNA.map((s) => ({ value: s, label: LABEL_STATUS[s] })),
  },
  {
    kunci: "mfa",
    label: "MFA",
    options: [
      { value: "true", label: "MFA aktif" },
      { value: "false", label: "MFA belum aktif" },
    ],
  },
  {
    kunci: "terkunci",
    label: "Kunci login",
    options: [
      { value: "true", label: "Terkunci" },
      { value: "false", label: "Tidak terkunci" },
    ],
  },
];

/**
 * Kontrol filter daftar pengguna (peran, lapis, status, MFA, kunci login). Setiap pilihan dijalankan server;
 * keadaannya ada di `state` (URL halaman), bukan di komponen ini.
 */
export function FilterPengguna({
  state,
  onFilter,
  onReset,
}: {
  state: StateTabelPengguna;
  onFilter: (kunci: KunciFilter, nilai: string | undefined) => void;
  onReset: () => void;
}) {
  const aktif = jumlahFilterAktif(state);
  return (
    <div
      role="group"
      aria-label="Filter pengguna"
      className="flex flex-wrap items-center gap-2"
    >
      {KONTROL.map((k) => (
        <Select
          key={k.kunci}
          value={state[k.kunci] ?? SEMUA}
          onValueChange={(v: string) =>
            onFilter(k.kunci, v === SEMUA ? undefined : v)
          }
        >
          <SelectTrigger
            aria-label={`Filter ${k.label.toLowerCase()}`}
            className={cn(
              "w-[calc(50%-0.25rem)] text-[13px] sm:w-44 md:text-[13px]",
              k.kunci === "terkunci" && "max-sm:w-full",
            )}
          >
            <SelectValue placeholder={k.label} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={SEMUA}>{k.label}: semua</SelectItem>
            {k.options.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ))}
      {aktif > 0 && (
        <Button
          variant="ghost"
          size="sm"
          className="h-9 text-[13px]"
          onClick={onReset}
        >
          <X /> Reset filter ({aktif})
        </Button>
      )}
    </div>
  );
}
