"use client";

import { useEffect, useState } from "react";
import { ShieldCheck } from "lucide-react";
import { toast } from "sonner";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { SpringButton } from "@/components/garda/spring-button";
import { fetchKodeEtik, setujuiKodeEtik, type KodeEtikResponse } from "@/lib/api/lapangan";

const STORAGE_PREFIX = "garda:kode-etik";

function storageKey(userId: string, versi: string): string {
  return `${STORAGE_PREFIX}:${userId}:${versi}`;
}

interface KodeEtikModalProps {
  userId?: string;
  online: boolean;
  onDisetujui?: () => void;
}

/**
 * Menampilkan modal persetujuan kode etik pencacah saat petugas pertama kali
 * membuka form input apabila belum menyetujui versi aktif di server.
 */
export function KodeEtikModal({ userId, online, onDisetujui }: KodeEtikModalProps) {
  const [kodeEtik, setKodeEtik] = useState<KodeEtikResponse | null>(null);
  const [buka, setBuka] = useState(false);
  const [centang, setCentang] = useState(false);
  const [mengirim, setMengirim] = useState(false);

  useEffect(() => {
    if (!online || !userId) return;
    let batal = false;

    fetchKodeEtik()
      .then((res) => {
        if (batal) return;
        if (res.sudah_setuju) {
          try {
            localStorage.setItem(storageKey(userId, res.versi), "1");
          } catch {}
          onDisetujui?.();
          return;
        }
        try {
          if (localStorage.getItem(storageKey(userId, res.versi)) === "1") {
            return;
          }
        } catch {}
        setKodeEtik(res);
        setBuka(true);
      })
      .catch(() => {
        // Saat offline atau gagal memuat, jangan blokir form input;
        // kotak centang kode etik inline tetap menjadi pengaman.
      });

    return () => {
      batal = true;
    };
  }, [online, userId, onDisetujui]);

  async function handleSetuju() {
    if (!kodeEtik || !centang) return;
    setMengirim(true);
    try {
      await setujuiKodeEtik(kodeEtik.versi);
      if (userId) {
        try {
          localStorage.setItem(storageKey(userId, kodeEtik.versi), "1");
        } catch {}
      }
      toast.success("Persetujuan kode etik tercatat", {
        description: `Versi ${kodeEtik.versi} aktif untuk sesi pencacahan Anda.`,
      });
      setBuka(false);
      onDisetujui?.();
    } catch {
      toast.error("Gagal menyimpan persetujuan kode etik", {
        description: "Periksa koneksi internet Anda lalu coba kembali.",
      });
    } finally {
      setMengirim(false);
    }
  }

  return (
    <Dialog open={buka} onOpenChange={(open) => { if (!open && !mengirim) setBuka(false); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-base">
            <ShieldCheck className="size-5 shrink-0 text-data-600" />
            <span className="min-w-0">{kodeEtik?.judul ?? "Kode Etik Pencacah Lapangan"}</span>
          </DialogTitle>
          <DialogDescription>
            Versi {kodeEtik?.versi ?? "v1.0"} · Wajib disetujui sebelum melakukan pencacahan harga pangan.
          </DialogDescription>
        </DialogHeader>

        {/* Hanya teks kode etik yang digulir: kotak persetujuan dan tombol selalu terlihat. */}
        <DialogBody className="mx-0 my-0 rounded-lg border border-border bg-secondary p-3 text-xs leading-relaxed break-words whitespace-pre-line text-foreground sm:mx-0 sm:p-3">
          {kodeEtik?.isi}
        </DialogBody>

        <label className="flex shrink-0 cursor-pointer items-start gap-2.5 rounded-lg border border-border bg-background p-3 text-xs">
          <Checkbox
            checked={centang}
            onCheckedChange={(v: boolean | "indeterminate") => setCentang(v === true)}
            className="mt-0.5"
          />
          <span>
            Saya telah membaca, memahami, dan bersedia mematuhi seluruh butir kode etik pencacahan harga pangan di atas.
          </span>
        </label>

        <DialogFooter>
          <SpringButton
            type="button"
            variant="primary"
            size="lg"
            className="w-full"
            disabled={!centang || mengirim}
            onClick={() => void handleSetuju()}
          >
            {mengirim ? "Menyimpan..." : "Setujui & Lanjutkan"}
          </SpringButton>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
