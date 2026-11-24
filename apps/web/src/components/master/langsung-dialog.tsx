"use client";

import { useEffect, useState } from "react";
import { FormDialog } from "@/components/form/form-bff";
import { Button } from "@/components/ui/button";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { EntitasLangsung, LokasiSpasialRow } from "@/lib/api/master-langsung";
import type { MasterKomoditasRow, MasterPasarRow } from "@/lib/data/types";
import {
  useMasterPasar,
  useMasterWilayah,
  useSatuan,
  useSimpanMasterLangsung,
} from "@/lib/query/hooks";
import { KoordinatPicker } from "./koordinat-picker";

export type TargetLangsung =
  | { entitas: "komoditas"; awal?: MasterKomoditasRow }
  | { entitas: "pasar"; awal?: MasterPasarRow }
  | { entitas: "kecamatan" }
  | { entitas: "satuan" }
  | { entitas: "pedagang" }
  | { entitas: "pertanian"; awal?: LokasiSpasialRow }
  | { entitas: "perkampungan"; awal?: LokasiSpasialRow };

const LABEL_ENTITAS: Record<EntitasLangsung, string> = {
  komoditas: "Komoditas",
  pasar: "Pasar",
  kecamatan: "Kecamatan",
  satuan: "Satuan",
  pedagang: "Pedagang",
  pertanian: "Sentra pertanian",
  perkampungan: "Perkampungan",
};

export function LangsungDialog({
  target,
  open,
  onOpenChange,
}: {
  target: TargetLangsung | null;
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const wilayah = useMasterWilayah();
  const satuan = useSatuan();
  const pasarList = useMasterPasar();
  const simpan = useSimpanMasterLangsung();

  const [kode, setKode] = useState("");
  const [nama, setNama] = useState("");
  const [kategori, setKategori] = useState("Hortikultura");
  const [kodeSatuan, setKodeSatuan] = useState("kg");
  const [hargaAcuan, setHargaAcuan] = useState(25000);
  const [shelfLife, setShelfLife] = useState(14);
  const [pilot, setPilot] = useState(false);

  const [kabupatenKode, setKabupatenKode] = useState("1171");
  const [kodeDagri, setKodeDagri] = useState("");
  const [pasarId, setPasarId] = useState("");
  const [radiusMeter, setRadiusMeter] = useState(400);
  const [aktif, setAktif] = useState(true);

  const [komoditasUtama, setKomoditasUtama] = useState("");
  const [luasHa, setLuasHa] = useState<string>("");
  const [estimasiPanen, setEstimasiPanen] = useState<string>("");
  const [populasi, setPopulasi] = useState<string>("");
  const [lat, setLat] = useState(5.5577);
  const [lng, setLng] = useState(95.3222);

  useEffect(() => {
    if (!target || !open) return;
    if (target.entitas === "komoditas" && target.awal) {
      setKode(target.awal.kode);
      setNama(target.awal.nama);
      setKategori(target.awal.kategori);
      setKodeSatuan(target.awal.satuan);
      setHargaAcuan(target.awal.hargaAcuan);
      setShelfLife(target.awal.shelfLifeHari);
      setPilot(target.awal.pilot);
      return;
    }
    if (target.entitas === "pasar" && target.awal) {
      setKode(target.awal.kode ?? target.awal.id);
      setNama(target.awal.nama);
      setKabupatenKode(target.awal.kabupatenKode ?? "1171");
      setLat(target.awal.lat);
      setLng(target.awal.lng);
      setRadiusMeter(target.awal.radiusMeter);
      setAktif(target.awal.aktif);
      return;
    }
    if (
      (target.entitas === "pertanian" || target.entitas === "perkampungan") &&
      target.awal
    ) {
      setKode(target.awal.kode);
      setNama(target.awal.nama);
      setKabupatenKode(target.awal.kabupaten_kode);
      setKomoditasUtama(target.awal.komoditas_utama);
      setLuasHa(target.awal.luas_ha != null ? String(target.awal.luas_ha) : "");
      setEstimasiPanen(
        target.awal.estimasi_panen_ton != null
          ? String(target.awal.estimasi_panen_ton)
          : "",
      );
      setPopulasi(
        target.awal.populasi != null ? String(target.awal.populasi) : "",
      );
      setLat(target.awal.lat);
      setLng(target.awal.lng);
      setAktif(target.awal.aktif);
      return;
    }
    setKode("");
    setNama("");
    setKategori("Hortikultura");
    setKodeSatuan(satuan.data?.[0]?.kode ?? "kg");
    setHargaAcuan(25000);
    setShelfLife(14);
    setPilot(false);
    setKabupatenKode(wilayah.data?.[0]?.kode ?? "1171");
    setKodeDagri("");
    setPasarId(pasarList.data?.[0]?.kode ?? pasarList.data?.[0]?.id ?? "");
    setRadiusMeter(400);
    setAktif(true);
    setKomoditasUtama("");
    setLuasHa("");
    setEstimasiPanen("");
    setPopulasi("");
    setLat(5.5577);
    setLng(95.3222);
  }, [target, open, satuan.data, wilayah.data, pasarList.data]);

  if (!target) return null;

  const isEdit =
    ("awal" in target && Boolean(target.awal)) || false;
  const kodeEdit =
    target.entitas === "komoditas" && target.awal
      ? target.awal.kode
      : target.entitas === "pasar" && target.awal
        ? (target.awal.kode ?? target.awal.id)
        : (target.entitas === "pertanian" || target.entitas === "perkampungan") &&
            target.awal
          ? target.awal.kode
          : undefined;

  async function submit() {
    if (!target || !kode.trim() || !nama.trim()) return;
    let payload: Record<string, unknown> = {};
    switch (target.entitas) {
      case "komoditas":
        payload = {
          kode: kode.trim().toUpperCase(),
          nama: nama.trim(),
          kategori: kategori.trim(),
          satuan: kodeSatuan,
          harga_acuan: Number(hargaAcuan),
          shelf_life_hari: Number(shelfLife),
          pilot,
          status: "AKTIF",
        };
        break;
      case "pasar":
        payload = {
          kode: kode.trim(),
          nama: nama.trim(),
          kabupaten_kode: kabupatenKode,
          lat: Number(lat),
          lng: Number(lng),
          radius_meter: Number(radiusMeter) || 400,
          aktif,
        };
        break;
      case "kecamatan":
        payload = {
          kode: kode.trim(),
          nama: nama.trim(),
          kabupaten_kode: kabupatenKode,
          kode_dagri: kodeDagri.trim() || null,
        };
        break;
      case "satuan":
        payload = {
          kode: kode.trim().toLowerCase(),
          nama: nama.trim(),
        };
        break;
      case "pedagang":
        payload = {
          kode: kode.trim(),
          nama: nama.trim(),
          pasar_id: pasarId,
          aktif,
        };
        break;
      case "pertanian":
      case "perkampungan":
        payload = {
          kode: kode.trim().toUpperCase(),
          nama: nama.trim(),
          kabupaten_kode: kabupatenKode,
          komoditas_utama: komoditasUtama.trim(),
          luas_ha: luasHa ? Number(luasHa) : null,
          estimasi_panen_ton: estimasiPanen ? Number(estimasiPanen) : null,
          populasi: populasi ? Number(populasi) : null,
          lat: Number(lat),
          lng: Number(lng),
          aktif,
        };
        break;
    }
    await simpan.mutateAsync({
      entitas: target.entitas,
      payload,
      kodeEdit,
    });
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <FormDialog onKirim={() => void submit()} sibuk={simpan.isPending}>
          <DialogHeader>
            <DialogTitle>
              {isEdit ? "Ubah" : "Tambah"} {LABEL_ENTITAS[target.entitas]}
            </DialogTitle>
            <DialogDescription>
              Mode Superadmin: perubahan disimpan langsung dan dicatat ke audit
              trail tanpa antrean persetujuan.
            </DialogDescription>
          </DialogHeader>

          <DialogBody className="space-y-3 max-h-[68dvh] overflow-y-auto py-3">
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div>
                <Label className="mb-1 block text-xs">Kode</Label>
                <Input
                  required
                  disabled={isEdit}
                  value={kode}
                  onChange={(e) => setKode(e.target.value)}
                  placeholder={
                    target.entitas === "pertanian"
                      ? "STN-1108-02"
                      : target.entitas === "perkampungan"
                        ? "KMP-1171-02"
                        : target.entitas === "pasar"
                          ? "PSR-1171-03"
                          : "KODE"
                  }
                />
              </div>
              <div>
                <Label className="mb-1 block text-xs">Nama</Label>
                <Input
                  required
                  value={nama}
                  onChange={(e) => setNama(e.target.value)}
                  placeholder={`Nama ${LABEL_ENTITAS[target.entitas].toLowerCase()}`}
                />
              </div>
            </div>

            {target.entitas === "komoditas" && (
              <>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <div>
                    <Label className="mb-1 block text-xs">Kategori</Label>
                    <Input
                      required
                      value={kategori}
                      onChange={(e) => setKategori(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label className="mb-1 block text-xs">Satuan</Label>
                    <Select value={kodeSatuan} onValueChange={setKodeSatuan}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {(satuan.data ?? [{ kode: "kg", nama: "Kilogram" }]).map(
                          (s) => (
                            <SelectItem key={s.kode} value={s.kode}>
                              {s.kode} · {s.nama}
                            </SelectItem>
                          ),
                        )}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <div>
                    <Label className="mb-1 block text-xs">
                      Harga acuan (Rp)
                    </Label>
                    <Input
                      type="number"
                      min={0}
                      value={hargaAcuan}
                      onChange={(e) => setHargaAcuan(Number(e.target.value))}
                    />
                  </div>
                  <div>
                    <Label className="mb-1 block text-xs">
                      Masa simpan (hari)
                    </Label>
                    <Input
                      type="number"
                      min={1}
                      value={shelfLife}
                      onChange={(e) => setShelfLife(Number(e.target.value))}
                    />
                  </div>
                </div>
                <label className="flex items-center gap-2 text-xs">
                  <Checkbox
                    checked={pilot}
                    onCheckedChange={(v) => setPilot(v === true)}
                  />
                  Tandai komoditas pilot pemantauan prioritas
                </label>
              </>
            )}

            {(target.entitas === "pasar" ||
              target.entitas === "kecamatan" ||
              target.entitas === "pertanian" ||
              target.entitas === "perkampungan") && (
              <div>
                <Label className="mb-1 block text-xs">Kabupaten/Kota</Label>
                <Select value={kabupatenKode} onValueChange={setKabupatenKode}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(wilayah.data ?? []).map((w) => (
                      <SelectItem key={w.kode} value={w.kode}>
                        {w.kode} · {w.nama}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {target.entitas === "kecamatan" && (
              <div>
                <Label className="mb-1 block text-xs">Kode Kemendagri</Label>
                <Input
                  value={kodeDagri}
                  onChange={(e) => setKodeDagri(e.target.value)}
                  placeholder="Opsional"
                />
              </div>
            )}

            {target.entitas === "pedagang" && (
              <div>
                <Label className="mb-1 block text-xs">Pasar</Label>
                <Select value={pasarId} onValueChange={setPasarId}>
                  <SelectTrigger>
                    <SelectValue placeholder="Pilih pasar" />
                  </SelectTrigger>
                  <SelectContent>
                    {(pasarList.data ?? []).map((p) => (
                      <SelectItem key={p.id} value={p.kode ?? p.id}>
                        {p.nama} ({p.kabupaten})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {target.entitas === "pasar" && (
              <div>
                <Label className="mb-1 block text-xs">
                  Radius geofence (meter)
                </Label>
                <Input
                  type="number"
                  min={50}
                  value={radiusMeter}
                  onChange={(e) => setRadiusMeter(Number(e.target.value))}
                />
              </div>
            )}

            {(target.entitas === "pertanian" ||
              target.entitas === "perkampungan") && (
              <>
                <div>
                  <Label className="mb-1 block text-xs">
                    {target.entitas === "pertanian"
                      ? "Komoditas unggulan"
                      : "Karakteristik konsumsi / pangan"}
                  </Label>
                  <Input
                    value={komoditasUtama}
                    onChange={(e) => setKomoditasUtama(e.target.value)}
                    placeholder={
                      target.entitas === "pertanian"
                        ? "Cabai merah keriting & Tomat"
                        : "Permukiman & sentra pedagang"
                    }
                  />
                </div>
                {target.entitas === "pertanian" ? (
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <Label className="mb-1 block text-xs">Luas (Ha)</Label>
                      <Input
                        type="number"
                        step="0.1"
                        value={luasHa}
                        onChange={(e) => setLuasHa(e.target.value)}
                      />
                    </div>
                    <div>
                      <Label className="mb-1 block text-xs">
                        Estimasi panen (Ton)
                      </Label>
                      <Input
                        type="number"
                        step="0.1"
                        value={estimasiPanen}
                        onChange={(e) => setEstimasiPanen(e.target.value)}
                      />
                    </div>
                  </div>
                ) : (
                  <div>
                    <Label className="mb-1 block text-xs">
                      Populasi (Jiwa)
                    </Label>
                    <Input
                      type="number"
                      value={populasi}
                      onChange={(e) => setPopulasi(e.target.value)}
                    />
                  </div>
                )}
              </>
            )}

            {(target.entitas === "pasar" ||
              target.entitas === "pertanian" ||
              target.entitas === "perkampungan") && (
              <KoordinatPicker
                lat={lat}
                lng={lng}
                onChange={(newLat, newLng) => {
                  setLat(newLat);
                  setLng(newLng);
                }}
              />
            )}
          </DialogBody>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Batal
            </Button>
            <Button type="submit" disabled={simpan.isPending}>
              {simpan.isPending ? "Menyimpan..." : "Simpan langsung"}
            </Button>
          </DialogFooter>
        </FormDialog>
      </DialogContent>
    </Dialog>
  );
}
