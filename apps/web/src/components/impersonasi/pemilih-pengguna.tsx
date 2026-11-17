"use client";

import { useQuery } from "@tanstack/react-query";
import { Search } from "lucide-react";
import { useState } from "react";
import { Input } from "@/components/ui/input";
import { cariPengguna, labelPeran, type PenggunaCari } from "@/lib/api/impersonasi";
import { useDebouncedValue } from "@/lib/use-debounced-value";

/**
 * Pencarian akun aktif (selain Administrator Sistem) untuk editor kebijakan. Ketik minimal dua huruf; memilih
 * hasil memanggil `onPilih` dan mengosongkan isian. Akun yang sudah dipilih dapat disembunyikan lewat `kecuali`.
 */
export function PemilihPengguna({
  onPilih,
  kecuali = [],
  label,
  placeholder = "Cari nama atau email akun…",
}: {
  onPilih: (u: PenggunaCari) => void;
  kecuali?: readonly string[];
  label: string;
  placeholder?: string;
}) {
  const [teks, setTeks] = useState("");
  const kata = useDebouncedValue(teks, 250);
  const { data, isFetching, isError } = useQuery({
    queryKey: ["impersonasi", "cari-pengguna", kata.trim()],
    queryFn: () => cariPengguna(kata),
    enabled: kata.trim().length >= 2,
    staleTime: 30_000,
    retry: false,
  });
  const hasil = (data ?? []).filter((u) => !kecuali.includes(u.id));
  // Jeda ketik (debounce) dan permintaan yang berjalan sama-sama "sedang mencari": jangan klaim "tidak ada" dulu.
  const mencari = teks.trim() !== kata.trim() || isFetching;

  return (
    <div className="grid gap-1.5">
      <div className="relative">
        <Search
          className="pointer-events-none absolute top-2.5 left-2.5 size-4 text-muted-foreground"
          aria-hidden
        />
        <Input
          value={teks}
          onChange={(e) => setTeks(e.target.value)}
          placeholder={placeholder}
          aria-label={label}
          className="pl-8"
        />
      </div>
      {teks.trim().length >= 2 && (
        <ul
          aria-label={`Hasil ${label.toLowerCase()}`}
          className="scrollbar-thin grid max-h-48 gap-0.5 overflow-y-auto rounded-lg border border-border bg-card p-1"
        >
          {isError ? (
            <li role="alert" className="px-2 py-1.5 text-sm text-destructive">
              Pencarian gagal. Coba lagi.
            </li>
          ) : hasil.length === 0 ? (
            <li className="px-2 py-1.5 text-sm text-muted-foreground">
              {mencari ? "Mencari…" : "Tidak ada akun yang cocok."}
            </li>
          ) : (
            hasil.map((u) => (
              <li key={u.id}>
                <button
                  type="button"
                  onClick={() => {
                    onPilih(u);
                    setTeks("");
                  }}
                  className="flex w-full flex-col rounded-md px-2 py-1.5 text-left outline-none hover:bg-secondary focus-visible:ring-2 focus-visible:ring-ring/40"
                >
                  <span className="text-[13px] font-medium">{u.nama}</span>
                  <span className="text-xs text-muted-foreground">
                    {u.email} · {labelPeran(u.peran)}
                  </span>
                </button>
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}
