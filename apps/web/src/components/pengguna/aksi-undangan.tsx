"use client";

import { Download, Mail, Send, Upload } from "lucide-react";
import { useState } from "react";
import { SpringButton } from "@/components/garda/spring-button";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import type { FilterEkspor } from "@/lib/api/pengguna-undangan";
import type { PenggunaRow } from "@/lib/data/types";
import {
  useEksporPengguna,
  useUndangUlangPengguna,
} from "@/lib/query/undangan-hooks";
import { ImporCsvDialog } from "./impor-csv-dialog";
import { UndangDialog } from "./undang-dialog";

/**
 * Aksi toolbar daftar pengguna untuk 02-D: "Undang pengguna" (cara utama menambah akun), "Impor CSV", dan
 * "Ekspor semua (CSV)" (dari server, tercatat di audit trail). Memiliki dialognya sendiri; `onBuatSementara`
 * membuka dialog akun dengan kata sandi sementara (opsi kedua). `filter` meneruskan pencarian aktif ke ekspor.
 */
export function AksiUndangan({
  onBuatSementara,
  filter,
}: {
  onBuatSementara?: () => void;
  filter?: FilterEkspor;
}) {
  const [dialog, setDialog] = useState<"undang" | "impor" | null>(null);
  const ekspor = useEksporPengguna();

  return (
    <>
      <SpringButton
        variant="primary"
        size="sm"
        onClick={() => setDialog("undang")}
      >
        <Mail /> Undang pengguna
      </SpringButton>
      <SpringButton variant="soft" size="sm" onClick={() => setDialog("impor")}>
        <Upload /> Impor CSV
      </SpringButton>
      <SpringButton
        variant="soft"
        size="sm"
        disabled={ekspor.pending}
        onClick={() => void ekspor.jalankan(filter)}
      >
        <Download /> {ekspor.pending ? "Mengekspor…" : "Ekspor semua (CSV)"}
      </SpringButton>

      <UndangDialog
        open={dialog === "undang"}
        onOpenChange={(v) => !v && setDialog(null)}
        onBuatSementara={
          onBuatSementara
            ? () => {
                setDialog(null);
                onBuatSementara();
              }
            : undefined
        }
      />
      <ImporCsvDialog
        open={dialog === "impor"}
        onOpenChange={(v) => !v && setDialog(null)}
      />
    </>
  );
}

/** Item menu baris "Kirim ulang undangan"; hanya dirender untuk akun yang menunggu aktivasi. */
export function ItemKirimUlangUndangan({
  user,
}: {
  user: Pick<PenggunaRow, "id" | "nama">;
}) {
  const ulang = useUndangUlangPengguna();
  return (
    <DropdownMenuItem
      onSelect={() => ulang.mutate({ id: user.id, nama: user.nama })}
    >
      <Send /> Kirim ulang undangan
    </DropdownMenuItem>
  );
}
