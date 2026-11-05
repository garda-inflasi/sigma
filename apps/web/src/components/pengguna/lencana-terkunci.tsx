"use client";

import { Lock } from "lucide-react";
import { StatusBadge } from "@/components/garda/status-badge";
import { useSekarang } from "@/components/auth/galat-auth";
import { formatSisaSingkat, sisaDetik } from "@/lib/auth/pesan-login";

/**
 * Lencana "Terkunci" untuk akun yang login-nya terkunci karena percobaan gagal berulang, dengan sisa
 * waktu yang berjalan. Hilang sendiri saat kunci berakhir. Sebelum jam klien diketahui (render server
 * dan hidrasi pertama) hanya "Terkunci" yang tampil, supaya HTML sama persis.
 */
export function LencanaTerkunci({ sampai }: { sampai?: string }) {
  const sampaiMs = sampai ? Date.parse(sampai) : Number.NaN;
  const valid = !Number.isNaN(sampaiMs);
  const sekarang = useSekarang(valid);
  if (!valid) return null;

  const sisa = sekarang === null ? null : sisaDetik(sampaiMs, sekarang);
  if (sisa !== null && sisa <= 0) return null;

  return (
    <StatusBadge tone="defisit" dot={false}>
      <Lock className="size-3" aria-hidden />
      Terkunci
      {sisa !== null && (
        <>
          <span aria-hidden>·</span>
          <span role="timer">{formatSisaSingkat(sisa)}</span>
        </>
      )}
    </StatusBadge>
  );
}
