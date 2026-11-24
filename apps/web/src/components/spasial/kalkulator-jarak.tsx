"use client";

import {
  AlertTriangle,
  ArrowRightLeft,
  Car,
  CheckCircle2,
  ChevronDown,
  Navigation,
  Plus,
  RotateCcw,
  Route,
  ShieldAlert,
  Truck,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { StatusBadge } from "@/components/garda/status-badge";
import { FormBff } from "@/components/form/form-bff";
import type {
  GarisUkurPeta,
  RuteGarisPeta,
} from "@/components/map/region-map-leaflet";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {
  JenisGangguanJalan,
  OpsiRuteJalan,
  TingkatGangguanJalan,
} from "@/lib/api/spasial-jarak";
import {
  estimasiJalanKm,
  estimasiMenitTempuh,
  haversineKm,
  risikoFoodLossDariMenit,
  type TitikPeta,
} from "@/lib/peta/aceh";
import {
  useCreateGangguanJalan,
  useGangguanJalan,
  useJarakSpasial,
  useUpdateGangguanJalan,
} from "@/lib/query/hooks";
import { useCan } from "@/lib/auth/session-context";
import { cn } from "@/lib/utils";

const JENIS_LABEL: Record<TitikPeta["jenis"], string> = {
  pertanian: "Sentra Pertanian",
  perkampungan: "Perkampungan",
  pasar: "Pasar",
  "cold-storage": "Cold Storage",
};

const JENIS_GANGGUAN_LABEL: Record<JenisGangguanJalan, string> = {
  LONGSOR: "Tanah Longsor",
  BANJIR: "Banjir / Genangan",
  JEMBATAN_PUTUS: "Jembatan Putus",
  JALAN_RUSAK: "Jalan Rusak Berat",
  PENUTUPAN: "Penutupan Jalan",
};

function keRuteGaris(opt: OpsiRuteJalan): RuteGarisPeta {
  return {
    tipe: opt.tipe,
    ringkasanJalan: opt.ringkasan_jalan,
    jarakKm: opt.jarak_km,
    durasiMenit: opt.durasi_menit,
    terdampakGangguan: opt.terdampak_gangguan,
    selisihKm: opt.selisih_km,
    selisihMenit: opt.selisih_menit,
    coordinates: opt.geometry?.coordinates ?? [],
  };
}

function toneFoodLoss(risiko?: "RENDAH" | "SEDANG" | "TINGGI") {
  if (risiko === "TINGGI") return "defisit" as const;
  if (risiko === "SEDANG") return "waspada" as const;
  return "normal" as const;
}

/**
 * Kalkulator jarak jaringan jalan raya (Sentra Pertanian / Perkampungan <-> Pasar / Cold Storage)
 * dengan pemilihan Jalur Utama & Jalur Alternatif serta pengalihan otomatis saat ada gangguan/bencana jalan.
 */
export function KalkulatorJarakPanel({
  titikLain,
  garisUkur,
  onChangeGaris,
  ruteTerpilihIdx = 0,
  onSelectRute,
}: {
  titikLain: TitikPeta[];
  garisUkur: GarisUkurPeta | null;
  onChangeGaris: (g: GarisUkurPeta | null) => void;
  ruteTerpilihIdx?: number;
  onSelectRute?: (idx: number) => void;
}) {
  const [asalId, setAsalId] = useState<string>("");
  const [tujuanId, setTujuanId] = useState<string>("");
  const [profil, setProfil] = useState<"truck" | "auto">("truck");
  const [tampilGangguan, setTampilGangguan] = useState(false);
  const [tampilFormGangguan, setTampilFormGangguan] = useState(false);

  const [judulBaru, setJudulBaru] = useState("");
  const [jenisBaru, setJenisBaru] = useState<JenisGangguanJalan>("LONGSOR");
  const [tingkatBaru, setTingkatBaru] =
    useState<TingkatGangguanJalan>("TUTUP_TOTAL");
  const [ruasBaru, setRuasBaru] = useState("");
  const [latBaru, setLatBaru] = useState("");
  const [lngBaru, setLngBaru] = useState("");
  const [radiusBaru, setRadiusBaru] = useState("450");

  const bisaKelolaLogistik = useCan("logistik.kelola");
  const { data: gangguanAktif = [] } = useGangguanJalan(true);
  const buatGangguan = useCreateGangguanJalan();
  const ubahGangguan = useUpdateGangguanJalan();

  const daftarAsal = useMemo(
    () =>
      titikLain.filter(
        (t) =>
          t.jenis === "pertanian" ||
          t.jenis === "perkampungan" ||
          t.jenis === "pasar",
      ),
    [titikLain],
  );
  const daftarTujuan = useMemo(
    () =>
      titikLain.filter(
        (t) => t.jenis === "pasar" || t.jenis === "cold-storage",
      ),
    [titikLain],
  );

  const asal = useMemo(
    () => titikLain.find((t) => t.id === asalId),
    [titikLain, asalId],
  );
  const tujuan = useMemo(
    () => titikLain.find((t) => t.id === tujuanId),
    [titikLain, tujuanId],
  );

  const queryParams = useMemo(() => {
    if (!asal) return null;
    return {
      asalLat: asal.lat,
      asalLng: asal.lng,
      tujuanLat: tujuan?.lat,
      tujuanLng: tujuan?.lng,
      faktorJalan: 1.3,
      profil,
    };
  }, [asal, tujuan, profil]);

  const { data: hasilApi, isFetching: memuatRute } = useJarakSpasial(
    queryParams,
    Boolean(asal),
  );

  // Hitungan instan lokal sambil menunggu respons routing backend selesai.
  const estimasiCepat = useMemo(() => {
    if (!asal || !tujuan) return null;
    const lurus =
      Math.round(haversineKm(asal.lat, asal.lng, tujuan.lat, tujuan.lng) * 10) /
      10;
    const jalan = estimasiJalanKm(lurus, 1.3);
    const menit = estimasiMenitTempuh(jalan);
    return {
      lurus,
      jalan,
      menit,
      risiko: risikoFoodLossDariMenit(menit),
    };
  }, [asal, tujuan]);

  const daftarRute = hasilApi?.rute ?? [];
  const idxAman =
    daftarRute.length > 0 &&
    ruteTerpilihIdx >= 0 &&
    ruteTerpilihIdx < daftarRute.length
      ? ruteTerpilihIdx
      : 0;
  const ruteDipilih = daftarRute[idxAman];

  const sigRef = useRef<string>("");

  useEffect(() => {
    onSelectRute?.(0);
  }, [asalId, tujuanId, profil, onSelectRute]);

  useEffect(() => {
    if (!asal || !tujuan) {
      if (sigRef.current !== "" || garisUkur !== null) {
        sigRef.current = "";
        onChangeGaris(null);
      }
      return;
    }

    const ruteGaris = daftarRute.map(keRuteGaris);
    const blokirGaris = hasilApi?.rute_diblokir
      ? keRuteGaris(hasilApi.rute_diblokir)
      : null;
    const aktif = ruteDipilih;
    const km = aktif?.jarak_km ?? estimasiCepat?.jalan ?? 0;
    const mnt = aktif?.durasi_menit ?? estimasiCepat?.menit ?? 0;
    const label = aktif
      ? `${aktif.tipe} · ${km} km · ~${mnt} mnt`
      : `Estimasi ${km} km · ~${mnt} mnt`;

    const nextSig = JSON.stringify({
      a: [asal.lat, asal.lng],
      t: [tujuan.lat, tujuan.lng],
      label,
      idx: idxAman,
      src: hasilApi?.sumber_rute ?? "FAST",
      n: ruteGaris.length,
      blk: Boolean(blokirGaris),
      c0: ruteGaris[0]?.coordinates?.[1] ?? null,
    });

    if (sigRef.current !== nextSig) {
      sigRef.current = nextSig;
      onChangeGaris({
        asal: [asal.lat, asal.lng],
        tujuan: [tujuan.lat, tujuan.lng],
        label,
        rute: ruteGaris.length > 0 ? ruteGaris : undefined,
        ruteTerpilihIdx: idxAman,
        ruteDiblokir: blokirGaris,
        sumberRute: hasilApi?.sumber_rute,
      });
    }
  }, [
    asal,
    tujuan,
    daftarRute,
    ruteDipilih,
    idxAman,
    hasilApi?.rute_diblokir,
    hasilApi?.sumber_rute,
    estimasiCepat,
    garisUkur,
    onChangeGaris,
  ]);

  const pilihFasilitasDariSaran = (lat: number, lng: number, nama: string) => {
    const cocok = daftarTujuan.find(
      (t) =>
        Math.abs(t.lat - lat) < 0.0005 &&
        Math.abs(t.lng - lng) < 0.0005 &&
        t.nama === nama,
    );
    if (cocok) {
      setTujuanId(cocok.id);
    }
  };

  const isiKoordinatTengah = () => {
    if (ruteDipilih?.geometry?.coordinates?.length) {
      const coords = ruteDipilih.geometry.coordinates;
      const mid = coords[Math.floor(coords.length / 2)];
      setLngBaru(mid[0].toFixed(4));
      setLatBaru(mid[1].toFixed(4));
      return;
    }
    if (asal && tujuan) {
      setLatBaru(((asal.lat + tujuan.lat) / 2).toFixed(4));
      setLngBaru(((asal.lng + tujuan.lng) / 2).toFixed(4));
      return;
    }
    if (asal) {
      setLatBaru(asal.lat.toFixed(4));
      setLngBaru(asal.lng.toFixed(4));
    }
  };

  const jarakTampilKm =
    ruteDipilih?.jarak_km ??
    hasilApi?.jarak_jalan_km ??
    estimasiCepat?.jalan ??
    null;
  const durasiTampilMnt =
    ruteDipilih?.durasi_menit ??
    hasilApi?.estimasi_menit_tempuh ??
    hasilApi?.menit_tempuh ??
    estimasiCepat?.menit ??
    null;
  const risikoTampil =
    ruteDipilih?.risiko_food_loss ??
    hasilApi?.risiko_food_loss ??
    estimasiCepat?.risiko;

  return (
    <div className="max-h-[44vh] shrink-0 overflow-y-auto border-t border-border bg-card/90 px-3 py-2.5 text-xs">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex items-center gap-1.5 font-medium text-foreground">
            <Navigation className="size-3.5 text-primary" />
            <span>Rute jalan raya sentra pertanian ke pasar &amp; cold storage</span>
          </div>
          {hasilApi?.sumber_rute && tujuan && (
            <Badge
              variant="outline"
              className="h-5 gap-1 px-1.5 text-[10px] font-medium"
            >
              <Route className="size-2.5" />
              {hasilApi.sumber_rute === "VALHALLA_OSM"
                ? "Jaringan Jalan OSM (Valhalla)"
                : hasilApi.sumber_rute === "OSRM_OSM"
                  ? "Jaringan Jalan OSM"
                  : hasilApi.sumber_rute === "SIMULASI_JARINGAN_JALAN"
                    ? "Simulasi Jaringan Jalan"
                    : "Estimasi Haversine"}
            </Badge>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-1.5">
          <div className="inline-flex rounded-md border border-border bg-background p-0.5">
            <button
              type="button"
              onClick={() => setProfil("truck")}
              className={cn(
                "inline-flex items-center gap-1 rounded px-2 py-0.5 text-[11px] font-medium transition-colors",
                profil === "truck"
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:text-foreground",
              )}
              title="Rute angkutan barang / truk logistik"
            >
              <Truck className="size-3" />
              Truk
            </button>
            <button
              type="button"
              onClick={() => setProfil("auto")}
              className={cn(
                "inline-flex items-center gap-1 rounded px-2 py-0.5 text-[11px] font-medium transition-colors",
                profil === "auto"
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:text-foreground",
              )}
              title="Rute mobil bak / kendaraan umum"
            >
              <Car className="size-3" />
              Mobil Bak
            </button>
          </div>

          <Button
            type="button"
            variant={gangguanAktif.length > 0 ? "outline" : "ghost"}
            size="sm"
            className={cn(
              "h-6 gap-1 px-2 text-[11px]",
              gangguanAktif.length > 0 &&
                "border-destructive/40 text-destructive hover:bg-destructive/10",
            )}
            onClick={() => setTampilGangguan((v) => !v)}
          >
            <ShieldAlert className="size-3" />
            Gangguan ({gangguanAktif.length})
            <ChevronDown
              className={cn(
                "size-3 transition-transform",
                tampilGangguan && "rotate-180",
              )}
            />
          </Button>

          {(asalId || tujuanId) && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-6 gap-1 px-2 text-[11px] text-muted-foreground"
              onClick={() => {
                setAsalId("");
                setTujuanId("");
                onChangeGaris(null);
              }}
            >
              <RotateCcw className="size-3" /> Reset
            </Button>
          )}
        </div>
      </div>

      <div className="mt-2 grid gap-2 sm:grid-cols-[1fr_auto_1fr]">
        <Select value={asalId} onValueChange={setAsalId}>
          <SelectTrigger className="h-8 text-xs">
            <SelectValue placeholder="Pilih asal (sentra / perkampungan / pasar)..." />
          </SelectTrigger>
          <SelectContent className="z-[1000] max-h-60">
            {daftarAsal.map((t) => (
              <SelectItem key={t.id} value={t.id} className="text-xs">
                {t.nama} ({JENIS_LABEL[t.jenis]})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label="Tukar titik asal dan tujuan"
          className="h-8 px-2"
          disabled={!asalId && !tujuanId}
          onClick={() => {
            const a = asalId;
            setAsalId(tujuanId);
            setTujuanId(a);
          }}
        >
          <ArrowRightLeft className="size-3.5" />
        </Button>

        <Select value={tujuanId} onValueChange={setTujuanId}>
          <SelectTrigger className="h-8 text-xs">
            <SelectValue placeholder="Pilih tujuan (pasar / cold storage)..." />
          </SelectTrigger>
          <SelectContent className="z-[1000] max-h-60">
            {daftarTujuan.map((t) => (
              <SelectItem key={t.id} value={t.id} className="text-xs">
                {t.nama} ({JENIS_LABEL[t.jenis]})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Ringkasan metrik rute terpilih */}
      {asal && tujuan && jarakTampilKm !== null && durasiTampilMnt !== null && (
        <div className="mt-2.5 space-y-2">
          {hasilApi?.jalur_utama_diblokir && (
            <div className="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-2.5 py-2 text-[11px] text-destructive">
              <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
              <div className="space-y-0.5">
                <p className="font-semibold">
                  Jalur utama terblokir — rute otomatis dialihkan ke{" "}
                  {ruteDipilih?.tipe ?? "Jalur Pengalihan"}
                </p>
                {hasilApi.gangguan_terkait?.[0] && (
                  <p className="text-foreground/85">
                    {hasilApi.gangguan_terkait[0].judul} (
                    {hasilApi.gangguan_terkait[0].jenis.replaceAll("_", " ")})
                    {hasilApi.gangguan_terkait[0].ruas_jalan
                      ? ` di ${hasilApi.gangguan_terkait[0].ruas_jalan}`
                      : ""}
                    .
                  </p>
                )}
              </div>
            </div>
          )}

          <div className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-primary/25 bg-primary/5 px-2.5 py-1.5">
            <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
              <span>
                Jarak jalan raya:{" "}
                <strong className="num text-foreground">
                  {jarakTampilKm} km
                </strong>
              </span>
              <span>
                Estimasi tempuh:{" "}
                <strong className="num text-foreground">
                  ~{durasiTampilMnt} menit
                </strong>
              </span>
              {hasilApi?.jarak_lurus_km !== undefined && (
                <span className="text-muted-foreground">
                  Garis lurus:{" "}
                  <strong className="num">{hasilApi.jarak_lurus_km} km</strong>
                </span>
              )}
              {memuatRute && (
                <span className="text-[11px] text-muted-foreground">
                  Menyinkronkan graf jalan...
                </span>
              )}
            </div>
            {risikoTampil && (
              <StatusBadge tone={toneFoodLoss(risikoTampil)}>
                Risiko food loss: {risikoTampil}
              </StatusBadge>
            )}
          </div>

          {/* Kartu pilihan Jalur Utama / Pengalihan / Alternatif */}
          {daftarRute.length > 0 && (
            <div className="grid gap-1.5 sm:grid-cols-2 lg:grid-cols-3">
              {daftarRute.map((opt, idx) => {
                const aktif = idx === idxAman;
                const adalahPengalihan = opt.tipe === "Jalur Pengalihan";
                return (
                  <button
                    key={`${opt.tipe}-${idx}`}
                    type="button"
                    onClick={() => onSelectRute?.(idx)}
                    className={cn(
                      "flex flex-col items-start gap-1 rounded-md border px-2.5 py-1.5 text-left transition-colors",
                      aktif
                        ? adalahPengalihan
                          ? "border-emerald-600/60 bg-emerald-500/10"
                          : "border-primary bg-primary/10"
                        : "border-border bg-background/80 hover:bg-muted/60",
                    )}
                  >
                    <div className="flex w-full items-center justify-between gap-1.5">
                      <span
                        className={cn(
                          "text-[11px] font-semibold",
                          adalahPengalihan
                            ? "text-emerald-700 dark:text-emerald-400"
                            : aktif
                              ? "text-primary"
                              : "text-foreground",
                        )}
                      >
                        {opt.tipe}
                      </span>
                      <span className="num text-[11px] font-medium text-foreground">
                        {opt.jarak_km} km · {opt.durasi_menit} mnt
                      </span>
                    </div>
                    <span className="line-clamp-1 text-[10.5px] text-muted-foreground">
                      {opt.ringkasan_jalan}
                    </span>
                    {(opt.selisih_km !== undefined && opt.selisih_km !== 0) ||
                    opt.terdampak_gangguan ? (
                      <span className="text-[10px] text-amber-700 dark:text-amber-400">
                        {opt.terdampak_gangguan
                          ? "Melintasi titik gangguan (TERHAMBAT)"
                          : `+${opt.selisih_km} km / +${opt.selisih_menit ?? 0} mnt dari jalur normal`}
                      </span>
                    ) : null}
                  </button>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Rekomendasi fasilitas terdekat (berbasis matriks waktu tempuh jalan) */}
      {asal &&
        !tujuan &&
        ((hasilApi?.pasar_terdekat?.length ?? 0) > 0 ||
          (hasilApi?.cold_storage_terdekat?.length ?? 0) > 0) && (
          <div className="mt-2 flex flex-wrap items-center gap-1.5">
            <span className="text-[11px] text-muted-foreground">
              Fasilitas terdekat via jalan:
            </span>
            {[
              ...(hasilApi?.pasar_terdekat ?? []).slice(0, 2),
              ...(hasilApi?.cold_storage_terdekat ?? []).slice(0, 2),
            ].map((f) => {
              const mnt = f.estimasi_menit_tempuh ?? f.menit_tempuh ?? 0;
              return (
                <button
                  key={`${f.jenis ?? "F"}-${f.kode}`}
                  type="button"
                  onClick={() =>
                    pilihFasilitasDariSaran(f.lat, f.lng, f.nama)
                  }
                  className="inline-flex items-center gap-1 rounded-md border border-border bg-background px-2 py-0.5 text-[11px] hover:border-primary hover:bg-primary/5"
                >
                  <span className="font-medium text-foreground">{f.nama}</span>
                  <span className="num text-muted-foreground">
                    {f.jarak_jalan_km} km · {mnt}m
                  </span>
                </button>
              );
            })}
          </div>
        )}

      {/* Drawer gangguan & penutupan jalan */}
      {tampilGangguan && (
        <div className="mt-2.5 space-y-2 rounded-md border border-border bg-background/70 p-2.5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-[11px] font-semibold text-foreground">
              Daftar Gangguan &amp; Bencana Jalan Aktif
            </p>
            {bisaKelolaLogistik && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-6 gap-1 px-2 text-[11px]"
                onClick={() => {
                  if (!tampilFormGangguan) {
                    isiKoordinatTengah();
                  }
                  setTampilFormGangguan((v) => !v);
                }}
              >
                <Plus className="size-3" />
                {tampilFormGangguan ? "Tutup Form" : "Laporkan Gangguan"}
              </Button>
            )}
          </div>

          {gangguanAktif.length === 0 ? (
            <p className="text-[11px] text-muted-foreground">
              Seluruh koridor jalan Aceh saat ini terpantau lancar tanpa laporan penutupan total.
            </p>
          ) : (
            <div className="space-y-1.5">
              {gangguanAktif.map((g) => (
                <div
                  key={g.id}
                  className="flex flex-wrap items-center justify-between gap-2 rounded border border-border/80 bg-card px-2 py-1.5"
                >
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <Badge
                        variant={
                          g.tingkat === "TUTUP_TOTAL"
                            ? "destructive"
                            : "secondary"
                        }
                        className="h-4 px-1.5 text-[10px]"
                      >
                        {g.tingkat === "TUTUP_TOTAL"
                          ? "Tutup Total"
                          : "Terhambat"}
                      </Badge>
                      <span className="font-medium text-foreground">
                        {g.judul}
                      </span>
                    </div>
                    <p className="text-[10.5px] text-muted-foreground">
                      {JENIS_GANGGUAN_LABEL[g.jenis]} · Radius {g.radius_m} m
                      {g.ruas_jalan ? ` · ${g.ruas_jalan}` : ""}
                    </p>
                  </div>
                  {bisaKelolaLogistik && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-6 gap-1 px-2 text-[11px] text-emerald-700 hover:bg-emerald-500/10"
                      disabled={ubahGangguan.isPending}
                      onClick={() =>
                        ubahGangguan.mutate({
                          id: g.id,
                          payload: { aktif: false },
                        })
                      }
                    >
                      <CheckCircle2 className="size-3" /> Selesai
                    </Button>
                  )}
                </div>
              ))}
            </div>
          )}

          {bisaKelolaLogistik && tampilFormGangguan && (
            <FormBff
              className="grid gap-2 border-t border-border pt-2 sm:grid-cols-2 lg:grid-cols-3"
              sibuk={buatGangguan.isPending}
              onKirim={() => {
                const latNum = Number(latBaru);
                const lngNum = Number(lngBaru);
                const radNum = Number(radiusBaru) || 450;
                if (
                  !judulBaru.trim() ||
                  Number.isNaN(latNum) ||
                  Number.isNaN(lngNum)
                ) {
                  return;
                }
                buatGangguan.mutate(
                  {
                    judul: judulBaru.trim(),
                    jenis: jenisBaru,
                    tingkat: tingkatBaru,
                    ruas_jalan: ruasBaru.trim(),
                    lat: latNum,
                    lng: lngNum,
                    radius_m: radNum,
                  },
                  {
                    onSuccess: () => {
                      setJudulBaru("");
                      setRuasBaru("");
                      setTampilFormGangguan(false);
                    },
                  },
                );
              }}
            >
              <Input
                value={judulBaru}
                onChange={(e) => setJudulBaru(e.target.value)}
                placeholder="Judul gangguan (mis. Longsor Bireuen KM 42)"
                className="h-7 text-xs sm:col-span-2"
                required
              />
              <select
                value={jenisBaru}
                onChange={(e) =>
                  setJenisBaru(e.target.value as JenisGangguanJalan)
                }
                className="h-7 rounded-md border border-input bg-background px-2 text-xs"
              >
                <option value="LONGSOR">Tanah Longsor</option>
                <option value="BANJIR">Banjir / Genangan</option>
                <option value="JEMBATAN_PUTUS">Jembatan Putus</option>
                <option value="JALAN_RUSAK">Jalan Rusak Berat</option>
                <option value="PENUTUPAN">Penutupan Jalan</option>
              </select>
              <select
                value={tingkatBaru}
                onChange={(e) =>
                  setTingkatBaru(e.target.value as TingkatGangguanJalan)
                }
                className="h-7 rounded-md border border-input bg-background px-2 text-xs"
              >
                <option value="TUTUP_TOTAL">Tutup Total (Alihkan Rute)</option>
                <option value="TERHAMBAT">Terhambat (Peringatan)</option>
              </select>
              <Input
                value={ruasBaru}
                onChange={(e) => setRuasBaru(e.target.value)}
                placeholder="Ruas jalan terdampak"
                className="h-7 text-xs"
              />
              <div className="flex gap-1.5">
                <Input
                  value={latBaru}
                  onChange={(e) => setLatBaru(e.target.value)}
                  placeholder="Lat (4.918)"
                  className="h-7 text-xs"
                  required
                />
                <Input
                  value={lngBaru}
                  onChange={(e) => setLngBaru(e.target.value)}
                  placeholder="Lng (96.762)"
                  className="h-7 text-xs"
                  required
                />
              </div>
              <div className="flex items-center gap-1.5 sm:col-span-2 lg:col-span-3">
                <Input
                  value={radiusBaru}
                  onChange={(e) => setRadiusBaru(e.target.value)}
                  placeholder="Radius meter (450)"
                  className="h-7 w-28 text-xs"
                />
                <Button
                  type="submit"
                  size="sm"
                  className="h-7 px-3 text-xs"
                  disabled={buatGangguan.isPending}
                >
                  {buatGangguan.isPending
                    ? "Menyimpan..."
                    : "Simpan & Hitung Ulang Rute"}
                </Button>
              </div>
            </FormBff>
          )}
        </div>
      )}
    </div>
  );
}
