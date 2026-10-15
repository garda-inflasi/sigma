"use client";

import { AnimatePresence, motion } from "motion/react";
import { Camera, Check, CheckCircle2, Loader2, MapPin, MapPinOff, ShieldCheck, Trash2, TriangleAlert } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { Segmented } from "@/components/garda/segmented";
import { SpringButton } from "@/components/garda/spring-button";
import { useSession } from "@/lib/auth/session-context";
import { KOMODITAS, PASAR, komoditasByKode, pasarById } from "@/lib/data/reference";
import type { Kualitas, TipeHarga } from "@/lib/data/types";
import { formatAngka, formatPersen, formatRupiah, parseRupiah } from "@/lib/format";
import { popIn, spring } from "@/lib/motion/presets";
import { BATAS_EKSTREM, TOLERANSI, hargaAcuan, jarakMeter, kompresFoto } from "@/lib/offline/acuan";
import { useOnline } from "@/lib/offline/use-online";
import { useFieldQueue } from "@/lib/stores/field-queue-store";
import { cn } from "@/lib/utils";

type Geo = { lat: number; lng: number; akurasi: number };

function Kartu({ judul, children, className }: { judul: string; children: React.ReactNode; className?: string }) {
  return (
    <section className={cn("glass rounded-xl p-4", className)}>
      <h2 className="mb-3 font-sans text-sm font-semibold text-navy-900 dark:text-white">{judul}</h2>
      {children}
    </section>
  );
}

/**
 * Form input harga per komoditas — offline-first.
 * Validasi di perangkat: rentang wajar (±25% median 7 hari), nilai ekstrem (salah satuan),
 * duplikasi harian, geofence pasar, bukti wajib bila di luar rentang, dan kode etik.
 */
export function PriceInputForm() {
  const user = useSession();
  const params = useSearchParams();
  const online = useOnline();
  const items = useFieldQueue((s) => s.items);
  const tambah = useFieldQueue((s) => s.tambah);

  const pasarList = user.kabupatenKode ? PASAR.filter((p) => p.kabupatenKode === user.kabupatenKode) : PASAR;
  const tugasKom = params.get("komoditas")?.split(",").filter((k) => komoditasByKode[k]);
  const daftarKom = tugasKom?.length ? tugasKom.map((k) => komoditasByKode[k]) : KOMODITAS;

  const [pasarId, setPasarId] = useState(params.get("pasar") ?? pasarList[0]?.id ?? PASAR[0].id);
  const [tipe, setTipe] = useState<TipeHarga>((params.get("tipe") as TipeHarga) ?? "KONSUMEN");
  const [kode, setKode] = useState(daftarKom[0].kode);
  const [hargaStr, setHargaStr] = useState("");
  const [kualitas, setKualitas] = useState<Kualitas>("MEDIUM");
  const [pedagang, setPedagang] = useState("");
  const [catatan, setCatatan] = useState("");
  const [foto, setFoto] = useState<string>();
  const [etik, setEtik] = useState(false);
  const [geo, setGeo] = useState<Geo | null>(null);
  const [geoErr, setGeoErr] = useState<string | null>(null);
  const [sukses, setSukses] = useState(false);

  useEffect(() => {
    if (!("geolocation" in navigator)) return setGeoErr("GPS tidak tersedia di perangkat ini");
    const id = navigator.geolocation.watchPosition(
      (p) => { setGeo({ lat: p.coords.latitude, lng: p.coords.longitude, akurasi: Math.round(p.coords.accuracy) }); setGeoErr(null); },
      (e) => setGeoErr(e.code === 1 ? "Izin lokasi ditolak. Aktifkan di pengaturan peramban." : "Lokasi belum terkunci"),
      { enableHighAccuracy: true, maximumAge: 30_000, timeout: 15_000 },
    );
    return () => navigator.geolocation.clearWatch(id);
  }, []);

  const kom = komoditasByKode[kode];
  const pasar = pasarById[pasarId];
  const acuan = useMemo(() => hargaAcuan(kode, pasarId, tipe), [kode, pasarId, tipe]);
  const harga = parseRupiah(hargaStr);
  const dev = harga ? ((harga - acuan.median) / acuan.median) * 100 : 0;
  const level = !harga ? "kosong" : Math.abs(dev) > BATAS_EKSTREM * 100 ? "ekstrem" : Math.abs(dev) > TOLERANSI * 100 ? "luar" : "wajar";
  const jarak = geo ? jarakMeter(geo, pasar) : null;
  const dalamGeofence = jarak === null ? undefined : jarak <= pasar.radiusMeter;
  const hariIni = new Date().toDateString();
  const sudahDiisi = (k: string) => items.some((i) => i.komoditasKode === k && i.pasarId === pasarId && i.tipe === tipe && new Date(i.dicatatPada).toDateString() === hariIni);
  const duplikat = sudahDiisi(kode);
  const butuhBukti = level === "luar" || level === "ekstrem";
  const buktiOk = !butuhBukti || !!foto || catatan.trim().length >= 10;
  const valid = harga > 0 && etik && buktiOk;
  const markerPos = harga ? Math.max(0, Math.min(100, ((harga / acuan.median - 0.5) / 1) * 100)) : 50;

  function simpan(status: "ANTRE" | "DRAF") {
    if (status === "ANTRE" && !valid) return;
    if (!harga) return toast.error("Isi harga terlebih dahulu");
    tambah({
      localId: crypto.randomUUID(), komoditasKode: kode, pasarId, tipe, harga, satuan: kom.satuan, kualitas,
      kodePedagang: pedagang || undefined, foto, lat: geo?.lat, lng: geo?.lng, akurasiM: geo?.akurasi, dalamGeofence,
      catatan: catatan.trim() || undefined, dicatatPada: new Date().toISOString(), status, percobaan: 0,
    });
    if (status === "ANTRE" && online) window.dispatchEvent(new CustomEvent("garda:sinkron"));
    toast.success(status === "DRAF" ? "Draf disimpan di perangkat" : online ? "Entri disimpan dan sedang dikirim" : "Entri disimpan offline", {
      description: status === "ANTRE" && !online ? "Akan dikirim otomatis saat jaringan kembali." : `${kom.nama}, ${formatRupiah(harga)}/${kom.satuan}`,
    });
    setSukses(true);
    setTimeout(() => setSukses(false), 1100);
    setHargaStr(""); setCatatan(""); setFoto(undefined); setPedagang("");
    const next = daftarKom.find((k) => k.kode !== kode && !sudahDiisi(k.kode));
    if (next) setKode(next.kode);
  }

  return (
    <div className="space-y-4">
      {tugasKom?.length ? (
        <p className="rounded-xl bg-data-50/80 px-4 py-2.5 text-sm text-data-800 dark:bg-data-500/10 dark:text-data-200">
          Tugas {params.get("tugas")}: <span className="num font-semibold">{daftarKom.filter((k) => sudahDiisi(k.kode)).length}/{daftarKom.length}</span> komoditas sudah dicatat hari ini.
        </p>
      ) : null}

      {/* Komoditas */}
      <div className="scrollbar-thin -mx-4 flex gap-2 overflow-x-auto px-4 pb-1" role="radiogroup" aria-label="Komoditas">
        {daftarKom.map((k) => {
          const aktif = k.kode === kode;
          const done = sudahDiisi(k.kode);
          return (
            <button key={k.kode} role="radio" aria-checked={aktif} onClick={() => setKode(k.kode)} className={cn("relative flex h-11 shrink-0 items-center gap-2 rounded-xl px-3.5 text-sm font-medium transition-colors", aktif ? "text-white" : "glass-subtle text-foreground")}>
              {aktif && <motion.span layoutId="kom-pill" transition={spring.layout} className="absolute inset-0 rounded-xl bg-navy-800 shadow-ambient-sm dark:bg-data-600" />}
              <KomoditasIcon kode={k.kode} className="relative" />
              <span className="relative whitespace-nowrap">{k.nama}</span>
              {done && <Check className={cn("relative size-3.5", aktif ? "text-data-300" : "text-normal")} aria-label="Sudah dicatat" />}
            </button>
          );
        })}
      </div>

      <Kartu judul="Lokasi">
        <Select value={pasarId} onValueChange={setPasarId}>
          <SelectTrigger className="h-12 w-full rounded-xl bg-white/70 text-[15px] dark:bg-white/5"><SelectValue /></SelectTrigger>
          <SelectContent>{pasarList.map((p) => <SelectItem key={p.id} value={p.id}>{p.nama}</SelectItem>)}</SelectContent>
        </Select>
        <p className={cn("mt-2.5 flex items-center gap-2 text-xs", dalamGeofence === false ? "text-waspada-ink" : dalamGeofence ? "text-normal-ink dark:text-emerald-300" : "text-muted-foreground")}>
          {geoErr ? <><MapPinOff className="size-3.5" />{geoErr}</>
            : !geo ? <><Loader2 className="size-3.5 animate-spin" />Mengunci lokasi GPS…</>
              : dalamGeofence ? <><MapPin className="size-3.5" />Dalam radius pasar, akurasi <span className="num">{geo.akurasi} m</span></>
                : <><MapPinOff className="size-3.5" /><span className="num">{formatAngka((jarak ?? 0) / 1000, 1)} km</span> dari pasar. Entri akan ditandai untuk diverifikasi koordinator.</>}
        </p>
      </Kartu>

      <Kartu judul="Harga">
        <Segmented layoutId="tipe-harga" value={tipe} onChange={setTipe} className="w-full" options={[{ value: "KONSUMEN", label: "Konsumen (eceran)" }, { value: "PRODUSEN", label: "Produsen (petani)" }]} />
        <label className="mt-4 block">
          <span className="text-xs text-muted-foreground">Harga per {kom.satuan}, {kom.nama.toLowerCase()}</span>
          <div className={cn("mt-1.5 flex items-baseline gap-2 rounded-xl border-2 bg-white/70 px-4 py-3 transition-colors dark:bg-white/5", level === "ekstrem" ? "border-defisit" : level === "luar" ? "border-waspada" : "border-transparent focus-within:border-data-400")}>
            <span className="num text-xl text-muted-foreground">Rp</span>
            <input
              inputMode="numeric"
              autoComplete="off"
              value={hargaStr}
              onChange={(e) => { const n = parseRupiah(e.target.value); setHargaStr(n ? formatAngka(n) : ""); }}
              placeholder={formatAngka(acuan.median)}
              className="num w-full bg-transparent text-4xl font-semibold tracking-tight outline-none placeholder:text-mist-300 dark:placeholder:text-white/15"
              aria-describedby="acuan-harga"
            />
          </div>
        </label>
        {/* Pita rentang wajar dengan penanda pegas */}
        <div className="mt-4" aria-hidden>
          <div className="relative h-2 rounded-full bg-mist-200 dark:bg-white/10">
            <div className="absolute inset-y-0 rounded-full bg-normal/35" style={{ left: `${(0.5 - TOLERANSI) * 100}%`, right: `${(0.5 - TOLERANSI) * 100}%` }} />
            <motion.span className={cn("absolute top-1/2 size-4 -translate-x-1/2 -translate-y-1/2 rounded-full ring-4 ring-white shadow-ambient dark:ring-navy-900", level === "ekstrem" ? "bg-defisit" : level === "luar" ? "bg-waspada" : level === "wajar" ? "bg-normal" : "bg-mist-400")} animate={{ left: `${markerPos}%` }} transition={spring.snappy} />
          </div>
        </div>
        <p id="acuan-harga" className="mt-2.5 text-xs text-muted-foreground">
          Median 7 hari <span className="num text-foreground">{formatRupiah(acuan.median)}</span>, rentang wajar <span className="num">{formatRupiah(acuan.bawah)}–{formatRupiah(acuan.atas)}</span>
        </p>
        <AnimatePresence mode="popLayout">
          {level !== "kosong" && (
            <motion.p key={level} variants={popIn} initial="hidden" animate="show" exit="exit" className={cn("mt-2 inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium", level === "wajar" ? "bg-normal-soft text-normal-ink" : level === "luar" ? "bg-waspada-soft text-waspada-ink" : "bg-defisit-soft text-defisit-ink")}>
              {level === "wajar" ? <CheckCircle2 className="size-3.5" /> : <TriangleAlert className="size-3.5" />}
              <span className="num">{formatPersen(dev, { signed: true })}</span>
              {level === "wajar" ? "dalam rentang wajar" : level === "luar" ? "di luar rentang wajar" : "periksa kembali satuan harga"}
            </motion.p>
          )}
        </AnimatePresence>
        {duplikat && <p className="mt-2 text-xs text-waspada-ink">Komoditas ini sudah dicatat hari ini di pasar yang sama. Entri baru tetap disimpan sebagai pembanding.</p>}

        <div className="mt-4 grid gap-3">
          <Segmented layoutId="kualitas" size="sm" value={kualitas} onChange={setKualitas} className="w-full" options={[{ value: "PREMIUM", label: "Premium" }, { value: "MEDIUM", label: "Medium" }, { value: "ASALAN", label: "Asalan" }]} />
          <label className="block">
            <span className="text-xs text-muted-foreground">Kode pedagang (opsional)</span>
            <input value={pedagang} onChange={(e) => setPedagang(e.target.value.toUpperCase())} placeholder="Mis. PNY-017" className="num mt-1 h-11 w-full rounded-xl border border-input bg-white/70 px-3 text-sm outline-none focus:shadow-glow-data dark:bg-white/5" />
            <span className="mt-1 block text-[11px] text-muted-foreground">Gunakan kode, bukan nama atau nomor telepon pedagang.</span>
          </label>
        </div>
      </Kartu>

      <Kartu judul={butuhBukti ? "Bukti (wajib untuk harga di luar rentang)" : "Bukti"}>
        <div className="flex gap-3">
          {foto ? (
            <div className="relative size-20 shrink-0 overflow-hidden rounded-xl">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={foto} alt="Foto bukti harga" className="size-full object-cover" />
              <button onClick={() => setFoto(undefined)} className="absolute top-1 right-1 grid size-6 place-items-center rounded-full bg-navy-900/70 text-white" aria-label="Hapus foto"><Trash2 className="size-3" /></button>
            </div>
          ) : (
            <label className="grid size-20 shrink-0 cursor-pointer place-items-center rounded-xl border-2 border-dashed border-border text-muted-foreground hover:border-data-400 hover:text-data-600">
              <span className="grid justify-items-center gap-1 text-[11px]"><Camera className="size-5" />Foto</span>
              <input type="file" accept="image/*" capture="environment" className="sr-only" onChange={async (e) => { const f = e.target.files?.[0]; if (f) setFoto(await kompresFoto(f)); }} />
            </label>
          )}
          <textarea value={catatan} onChange={(e) => setCatatan(e.target.value)} rows={3} placeholder="Catatan: kondisi pasokan, alasan harga berbeda, dll." className="min-h-20 w-full flex-1 resize-none rounded-xl border border-input bg-white/70 px-3 py-2.5 text-sm outline-none focus:shadow-glow-data dark:bg-white/5" />
        </div>
        {butuhBukti && !buktiOk && <p className="mt-2 text-xs text-waspada-ink">Lampirkan foto atau catatan minimal 10 karakter agar koordinator dapat memverifikasi.</p>}
      </Kartu>

      <label className="glass-subtle flex cursor-pointer gap-3 rounded-xl p-4">
        <Checkbox checked={etik} onCheckedChange={(v: boolean | "indeterminate") => setEtik(v === true)} className="mt-0.5 rounded-md" />
        <span className="text-sm leading-relaxed">
          <span className="flex items-center gap-1.5 font-medium"><ShieldCheck className="size-4 text-data-600" />Kode etik agen lapangan</span>
          <span className="text-muted-foreground">Saya mencatat harga yang berlaku tanpa menentukan, menawar, atau menjadi perantara transaksi.</span>
        </span>
      </label>

      {/* Bilah aksi lengket di atas navigasi bawah */}
      <div className="fixed inset-x-0 bottom-[92px] z-30 mx-auto max-w-md px-4">
        <div className="glass-strong flex gap-2 rounded-xl p-2">
          <SpringButton variant="soft" size="lg" onClick={() => simpan("DRAF")} disabled={!harga}>Simpan draf</SpringButton>
          <SpringButton variant={online ? "primary" : "data"} size="lg" className="flex-1" onClick={() => simpan("ANTRE")} disabled={!valid}>
            {online ? "Simpan entri" : "Simpan offline"}
          </SpringButton>
        </div>
      </div>

      <AnimatePresence>
        {sukses && (
          <motion.div className="pointer-events-none fixed inset-0 z-50 grid place-items-center" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
            <motion.div variants={popIn} initial="hidden" animate="show" exit="exit" className="glass-strong grid size-28 place-items-center rounded-full">
              <CheckCircle2 className="size-12 text-normal" />
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
