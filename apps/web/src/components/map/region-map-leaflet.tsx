"use client";

import "leaflet/dist/leaflet.css";
import L from "leaflet";
import { useTheme } from "next-themes";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  geometriKeFeatureCollection,
  type GeometriWilayah,
} from "@/lib/api/spasial";
import {
  BATAS_ACEH,
  BATAS_GESER,
  META_STATUS,
  TILE_PETA,
  URL_GEOJSON_ACEH,
  formatDelta,
  type MarkerWilayah,
  type StatusWilayah,
  type TitikPeta,
} from "@/lib/peta/aceh";

type KoleksiFitur = GeoJSON.FeatureCollection<
  GeoJSON.Polygon | GeoJSON.MultiPolygon
>;

export interface RuteGarisPeta {
  tipe: string;
  ringkasanJalan?: string;
  jarakKm: number;
  durasiMenit: number;
  terdampakGangguan?: boolean;
  selisihKm?: number;
  selisihMenit?: number;
  /** GeoJSON [lng, lat][] dari engine routing */
  coordinates: [number, number][];
}

export interface GangguanPeta {
  id: string;
  kode: string;
  judul: string;
  jenis: string;
  tingkat: "TUTUP_TOTAL" | "TERHAMBAT";
  ruasJalan: string;
  lat: number;
  lng: number;
  radiusM: number;
  aktif: boolean;
}

export interface GarisUkurPeta {
  asal: [number, number];
  tujuan: [number, number];
  label?: string;
  rute?: RuteGarisPeta[];
  ruteTerpilihIdx?: number;
  ruteDiblokir?: RuteGarisPeta | null;
  sumberRute?: string;
}

export interface PropsPetaLeaflet {
  markers: MarkerWilayah[];
  geometri?: GeometriWilayah[];
  titikLain?: TitikPeta[];
  titikCari?: { lat: number; lng: number };
  garisUkur?: GarisUkurPeta | null;
  gangguanJalan?: GangguanPeta[];
  onSelectRute?: (idx: number) => void;
  selected?: string;
  onSelect?: (kode: string) => void;
  onKlikPeta?: (lat: number, lng: number) => void;
  onKlikTitik?: (titik: TitikPeta) => void;
  /** Tombol tambahan di kartu wilayah, mis. "Buka peta lengkap". */
  aksiPopup?: { label: string; onClick: (kode: string) => void };
  /** Geser peta ke wilayah saat `selected` berubah dari luar. Matikan untuk pilihan hasil klik peta. */
  fokusPilihan?: boolean;
}

const IKON_FIT =
  '<svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/></svg>';

const geserHalus = () =>
  typeof window === "undefined" ||
  !window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;

const statusAman = (s: string | undefined): StatusWilayah =>
  s && s in META_STATUS ? (s as StatusWilayah) : "normal";

/** Isi kartu dibangun dengan `textContent`: nama dari API tidak pernah masuk sebagai HTML. */
function elemen<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className: string,
  teks?: string,
) {
  const el = document.createElement(tag);
  el.className = className;
  if (teks !== undefined) el.textContent = teks;
  return el;
}

function barisStatus(status: StatusWilayah) {
  const meta = META_STATUS[status];
  const baris = elemen("span", "garda-peta-status");
  const titik = elemen("span", "garda-peta-titik");
  titik.style.backgroundColor = meta.warna;
  baris.append(titik, document.createTextNode(meta.label));
  baris.title = meta.arti;
  return baris;
}

function isiRingkas(nama: string, m?: MarkerWilayah) {
  const kartu = elemen("div", "garda-peta-kartu");
  kartu.append(elemen("p", "garda-peta-judul", nama));
  if (!m) {
    kartu.append(elemen("p", "garda-peta-teks", "Belum ada data harga"));
    return kartu;
  }
  const baris = elemen("div", "garda-peta-baris");
  baris.append(
    barisStatus(statusAman(m.status)),
    elemen("span", "garda-peta-angka num", formatDelta(m.delta)),
  );
  kartu.append(baris);
  if (m.komoditas)
    kartu.append(elemen("p", "garda-peta-teks", `Pemicu: ${m.komoditas}`));
  return kartu;
}

function tombolKartu(label: string, utama: boolean, onClick: () => void) {
  const b = elemen(
    "button",
    utama ? "garda-peta-aksi garda-peta-aksi-utama" : "garda-peta-aksi",
    label,
  );
  b.type = "button";
  L.DomEvent.on(b, "click", (e) => {
    L.DomEvent.stop(e);
    onClick();
  });
  return b;
}

const ikonWilayah = (
  status: StatusWilayah,
  punyaPasar: boolean,
  terpilih: boolean,
) =>
  L.divIcon({
    className: "garda-pin-wadah",
    html: `<span class="garda-pin${status === "defisit" ? " garda-pin-denyut" : ""}${punyaPasar ? "" : " garda-pin-kecil"}${terpilih ? " garda-pin-terpilih" : ""}" style="--pin:${META_STATUS[status].warna}"></span>`,
    iconSize: [18, 18],
    iconAnchor: [9, 9],
  });

const KELAS_PIN_TITIK: Record<TitikPeta["jenis"], string> = {
  pasar: "garda-pin-pasar",
  "cold-storage": "garda-pin-cold",
  pertanian: "garda-pin-pertanian",
  perkampungan: "garda-pin-perkampungan",
};

const ikonTitik = (jenis: TitikPeta["jenis"]) =>
  L.divIcon({
    className: "garda-pin-wadah",
    html: `<span class="${KELAS_PIN_TITIK[jenis] ?? "garda-pin-pasar"}"></span>`,
    iconSize: [12, 12],
    iconAnchor: [6, 6],
  });

const IKON_CARI = () =>
  L.divIcon({
    className: "garda-pin-wadah",
    html: '<span class="garda-pin-cari"></span>',
    iconSize: [26, 26],
    iconAnchor: [13, 13],
  });

/**
 * Peta Leaflet Provinsi Aceh. Dibungkus `dynamic(..., { ssr: false })` oleh `RegionMap` karena Leaflet menyentuh
 * `window` saat dimuat. Memakai API Leaflet langsung (tanpa react-leaflet) agar aman dengan React 19.
 */
export default function PetaLeaflet({
  markers,
  geometri,
  titikLain,
  titikCari,
  garisUkur,
  gangguanJalan,
  onSelectRute,
  selected,
  onSelect,
  onKlikPeta,
  onKlikTitik,
  aksiPopup,
  fokusPilihan = true,
}: PropsPetaLeaflet) {
  const { resolvedTheme } = useTheme();
  const [peta, setPeta] = useState<L.Map | null>(null);
  const [statis, setStatis] = useState<KoleksiFitur | null>(null);
  const lapisanRef = useRef<L.GeoJSON | null>(null);
  const lewatiFokus = useRef<string | undefined>(undefined);
  const hoverRef = useRef<string | undefined>(undefined);

  // Handler terbaru dibaca lewat ref supaya lapisan tidak dibangun ulang tiap render.
  const terbaru = useRef({
    onSelect,
    onKlikPeta,
    onKlikTitik,
    onSelectRute,
    aksiPopup,
    selected,
    fokusPilihan,
  });
  useEffect(() => {
    terbaru.current = {
      onSelect,
      onKlikPeta,
      onKlikTitik,
      onSelectRute,
      aksiPopup,
      selected,
      fokusPilihan,
    };
  });

  const markerByKode = useMemo(
    () => new Map(markers.map((m) => [m.kode, m])),
    [markers],
  );

  // Poligon API hanya dipakai bila batas resmi; poligon perkiraan lama diganti berkas batas BPS.
  const pakaiApi = !!geometri?.length && geometri.every((g) => !g.perkiraan);
  useEffect(() => {
    if (pakaiApi) return;
    const ctrl = new AbortController();
    fetch(URL_GEOJSON_ACEH, { signal: ctrl.signal })
      .then((r) => (r.ok ? (r.json() as Promise<KoleksiFitur>) : null))
      .then((fc) => {
        if (fc) setStatis(fc);
      })
      .catch(() => {});
    return () => ctrl.abort();
  }, [pakaiApi]);

  const fitur = useMemo<KoleksiFitur | null>(
    () =>
      pakaiApi
        ? (geometriKeFeatureCollection(
            geometri!,
            markers,
          ) as unknown as KoleksiFitur)
        : statis,
    [pakaiApi, geometri, markers, statis],
  );

  const pasangPeta = useCallback((el: HTMLDivElement | null) => {
    if (!el) return;
    const map = L.map(el, {
      zoomControl: false,
      minZoom: 5,
      maxZoom: 16,
      zoomSnap: 0.25,
      zoomDelta: 0.5,
      maxBounds: BATAS_GESER,
      maxBoundsViscosity: 0.85,
    });
    map.attributionControl.setPrefix(false);
    L.control
      .zoom({
        position: "topright",
        zoomInTitle: "Perbesar",
        zoomOutTitle: "Perkecil",
      })
      .addTo(map);

    const paskan = (animasi: boolean) =>
      map.fitBounds(BATAS_ACEH, {
        padding: [10, 10],
        animate: animasi && geserHalus(),
      });

    const KontrolFit = L.Control.extend({
      onAdd() {
        const wadah = L.DomUtil.create("div", "leaflet-bar garda-kontrol-fit");
        const tombol = L.DomUtil.create("button", "", wadah);
        tombol.type = "button";
        tombol.title = "Tampilkan seluruh Aceh";
        tombol.setAttribute("aria-label", "Tampilkan seluruh Aceh");
        tombol.innerHTML = IKON_FIT;
        L.DomEvent.disableClickPropagation(wadah);
        L.DomEvent.on(tombol, "click", () => {
          map.closePopup();
          paskan(true);
        });
        return wadah;
      },
    });
    new KontrolFit({ position: "topright" }).addTo(map);

    paskan(false);
    let sudahPas = el.clientWidth > 0 && el.clientHeight > 0;
    const pengamat =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(() => {
            if (!el.clientWidth || !el.clientHeight) return;
            map.invalidateSize({ animate: false });
            if (!sudahPas) {
              sudahPas = true;
              paskan(false);
            }
          });
    pengamat?.observe(el);

    map.on("click", (e: L.LeafletMouseEvent) => {
      terbaru.current.onKlikPeta?.(e.latlng.lat, e.latlng.lng);
    });

    setPeta(map);
    return () => {
      pengamat?.disconnect();
      setPeta(null);
      map.remove();
    };
  }, []);

  // Warna tile mengikuti tema lewat filter CSS; poligon tetap terbaca walau tile gagal dimuat.
  useEffect(() => {
    if (!peta) return;
    const tile = L.tileLayer(TILE_PETA.url, {
      attribution: TILE_PETA.atribusi,
      maxZoom: 19,
    }).addTo(peta);
    return () => {
      tile.remove();
    };
  }, [peta]);

  const gaya = useCallback(
    (kode: string): L.PathOptions => {
      const status = statusAman(markerByKode.get(kode)?.status);
      const aktif = kode === terbaru.current.selected;
      const hover = kode === hoverRef.current;
      const warna = META_STATUS[status].warna;
      return {
        color: aktif
          ? resolvedTheme === "dark"
            ? "#ffffff"
            : "#18181b"
          : warna,
        weight: aktif ? 2.25 : hover ? 1.75 : 0.9,
        opacity: aktif ? 1 : 0.85,
        fillColor: warna,
        fillOpacity: aktif ? 0.46 : hover ? 0.4 : 0.26,
      };
    },
    [markerByKode, resolvedTheme],
  );

  const bukaKartu = useCallback(
    (map: L.Map, kode: string, nama: string, latlng: L.LatLngExpression) => {
      const m = markerByKode.get(kode);
      const kartu = isiRingkas(m?.nama ?? nama, m);
      const aksi = elemen("div", "garda-peta-aksi-baris");
      aksi.append(
        tombolKartu("Fokus wilayah", !terbaru.current.aksiPopup, () => {
          const lapisan = lapisanRef.current
            ?.getLayers()
            .find(
              (l) =>
                (l as L.GeoJSON & { feature?: GeoJSON.Feature }).feature
                  ?.properties?.kode === kode,
            ) as L.Polygon | undefined;
          if (lapisan)
            map.flyToBounds(lapisan.getBounds(), {
              padding: [28, 28],
              maxZoom: 11,
              animate: geserHalus(),
              duration: 0.6,
            });
          terbaru.current.onSelect?.(kode);
        }),
      );
      const tambahan = terbaru.current.aksiPopup;
      if (tambahan)
        aksi.append(
          tombolKartu(tambahan.label, true, () => tambahan.onClick(kode)),
        );
      kartu.append(aksi);
      L.popup({
        className: "garda-peta-popup",
        closeButton: true,
        offset: [0, -4],
        maxWidth: 260,
        minWidth: 200,
      })
        .setLatLng(latlng)
        .setContent(kartu)
        .openOn(map);
    },
    [markerByKode],
  );

  // Poligon kabupaten/kota. `L.geoJSON` membaca [lng, lat] dan mengubahnya menjadi [lat, lng].
  useEffect(() => {
    if (!peta || !fitur) return;
    const kodeDari = (f?: GeoJSON.Feature) => String(f?.properties?.kode ?? "");
    const lapisan: L.GeoJSON = L.geoJSON(fitur, {
      style: (f) => gaya(kodeDari(f)),
      onEachFeature: (f, l) => {
        const kode = kodeDari(f);
        const nama = String(f.properties?.nama ?? kode);
        l.bindTooltip(
          () =>
            isiRingkas(
              markerByKode.get(kode)?.nama ?? nama,
              markerByKode.get(kode),
            ),
          {
            sticky: true,
            direction: "top",
            offset: [0, -6],
            className: "garda-peta-tooltip",
          },
        );
        l.on({
          mouseover: () => {
            hoverRef.current = kode;
            (l as L.Path).setStyle(gaya(kode));
          },
          mouseout: () => {
            hoverRef.current = undefined;
            (l as L.Path).setStyle(gaya(kode));
          },
          click: (e: L.LeafletMouseEvent) => {
            L.DomEvent.stopPropagation(e);
            l.closeTooltip();
            lewatiFokus.current = kode;
            terbaru.current.onSelect?.(kode);
            terbaru.current.onKlikPeta?.(e.latlng.lat, e.latlng.lng);
            bukaKartu(peta, kode, nama, e.latlng);
          },
        });
      },
    }).addTo(peta);
    lapisanRef.current = lapisan;
    return () => {
      lapisanRef.current = null;
      lapisan.remove();
    };
  }, [peta, fitur, gaya, bukaKartu, markerByKode]);

  // Sorotan dan fokus wilayah terpilih dari luar peta (daftar wilayah, tautan ?wilayah=).
  useEffect(() => {
    const lapisan = lapisanRef.current;
    if (!peta || !lapisan) return;
    lapisan.eachLayer((l) => {
      const kode = String(
        (l as L.Polygon & { feature?: GeoJSON.Feature }).feature?.properties
          ?.kode ?? "",
      );
      (l as L.Path).setStyle(gaya(kode));
      if (kode === selected) (l as L.Path).bringToFront();
    });
    if (!selected) return;
    if (!terbaru.current.fokusPilihan) return;
    if (lewatiFokus.current === selected) {
      lewatiFokus.current = undefined;
      return;
    }
    const target = lapisan
      .getLayers()
      .find(
        (l) =>
          (l as L.Polygon & { feature?: GeoJSON.Feature }).feature?.properties
            ?.kode === selected,
      ) as L.Polygon | undefined;
    if (target)
      peta.flyToBounds(target.getBounds(), {
        padding: [36, 36],
        maxZoom: 10,
        animate: geserHalus(),
        duration: 0.6,
      });
  }, [peta, fitur, selected, gaya]);

  // Pin pusat kabupaten/kota.
  useEffect(() => {
    if (!peta) return;
    const grup = L.layerGroup().addTo(peta);
    for (const m of markers) {
      const status = statusAman(m.status);
      const pin = L.marker([m.lat, m.lng], {
        icon: ikonWilayah(status, m.punyaPasar, m.kode === selected),
        keyboard: false,
        riseOnHover: true,
        zIndexOffset:
          m.kode === selected
            ? 600
            : META_STATUS[status].urutan === 0
              ? 300
              : 100,
      }).addTo(grup);
      pin.getElement()?.setAttribute("data-kode", m.kode);
      pin.bindTooltip(() => isiRingkas(m.nama, m), {
        direction: "top",
        offset: [0, -8],
        className: "garda-peta-tooltip",
      });
      pin.on("click", (e: L.LeafletMouseEvent) => {
        L.DomEvent.stopPropagation(e);
        pin.closeTooltip();
        lewatiFokus.current = m.kode;
        terbaru.current.onSelect?.(m.kode);
        bukaKartu(peta, m.kode, m.nama, [m.lat, m.lng]);
      });
    }
    return () => {
      grup.remove();
    };
  }, [peta, markers, selected, bukaKartu]);

  // Pasar dan cold storage.
  useEffect(() => {
    if (!peta || !titikLain?.length) return;
    const grup = L.layerGroup().addTo(peta);
    for (const t of titikLain) {
      const pin = L.marker([t.lat, t.lng], {
        icon: ikonTitik(t.jenis),
        keyboard: false,
        // Di bawah pin ibu kota: titik yang berdekatan tidak menghalangi klik wilayah pada zoom rendah.
        zIndexOffset: -200,
      }).addTo(grup);
      pin.getElement()?.setAttribute("data-jenis", t.jenis);
      const kartu = () => {
        const k = elemen("div", "garda-peta-kartu");
        k.append(
          elemen("p", "garda-peta-judul", t.nama),
          elemen(
            "p",
            "garda-peta-teks",
            t.keterangan ?? (t.jenis === "pasar" ? "Pasar" : "Cold storage"),
          ),
        );
        return k;
      };
      pin.bindTooltip(kartu, {
        direction: "top",
        offset: [0, -6],
        className: "garda-peta-tooltip",
      });
      pin.on("click", (e: L.LeafletMouseEvent) => {
        L.DomEvent.stopPropagation(e);
        pin.closeTooltip();
        terbaru.current.onKlikTitik?.(t);
        L.popup({
          className: "garda-peta-popup",
          offset: [0, -4],
          maxWidth: 240,
        })
          .setLatLng([t.lat, t.lng])
          .setContent(kartu())
          .openOn(peta);
      });
    }
    return () => {
      grup.remove();
    };
  }, [peta, titikLain]);

  // Garis pengukur jarak & rute jaringan jalan (utama / pengalihan / alternatif + gangguan jalan).
  useEffect(() => {
    if (!peta || (!garisUkur && !gangguanJalan?.length)) return;
    const grup = L.layerGroup().addTo(peta);
    let boundsTarget: L.Polyline | null = null;

    if (garisUkur) {
      const keLatLngs = (coords?: [number, number][]): [number, number][] =>
        coords && coords.length >= 2
          ? coords.map(([lng, lat]) => [lat, lng])
          : [garisUkur.asal, garisUkur.tujuan];

      if (garisUkur.ruteDiblokir?.coordinates?.length) {
        const blocked = L.polyline(keLatLngs(garisUkur.ruteDiblokir.coordinates), {
          color: "#dc2626",
          weight: 3.5,
          dashArray: "5 7",
          opacity: 0.78,
        }).addTo(grup);
        blocked.bindTooltip(
          `Jalur terdampak: ${garisUkur.ruteDiblokir.ringkasanJalan ?? "Ruas terblokir"} (${garisUkur.ruteDiblokir.jarakKm} km)`,
          { sticky: true, className: "garda-peta-tooltip" },
        );
      }

      const daftar = garisUkur.rute ?? [];
      if (daftar.length > 0) {
        const aktifIdx =
          typeof garisUkur.ruteTerpilihIdx === "number" &&
          garisUkur.ruteTerpilihIdx >= 0 &&
          garisUkur.ruteTerpilihIdx < daftar.length
            ? garisUkur.ruteTerpilihIdx
            : 0;

        daftar.forEach((opt, idx) => {
          if (idx === aktifIdx) return;
          const altLine = L.polyline(keLatLngs(opt.coordinates), {
            color: opt.terdampakGangguan ? "#d97706" : "#64748b",
            weight: 3,
            dashArray: "7 6",
            opacity: 0.72,
          }).addTo(grup);
          altLine.on("click", () => {
            terbaru.current.onSelectRute?.(idx);
          });
          altLine.bindTooltip(
            `${opt.tipe}: ${opt.jarakKm} km · ~${opt.durasiMenit} mnt${opt.ringkasanJalan ? ` (${opt.ringkasanJalan})` : ""} — klik untuk pilih`,
            { sticky: true, className: "garda-peta-tooltip" },
          );
        });

        const selectedOpt = daftar[aktifIdx];
        const selectedColor =
          selectedOpt.tipe === "Jalur Pengalihan"
            ? "#059669"
            : selectedOpt.terdampakGangguan
              ? "#d97706"
              : selectedOpt.tipe === "Jalur Alternatif"
                ? "#4f46e5"
                : "#0284c7";
        const mainLine = L.polyline(keLatLngs(selectedOpt.coordinates), {
          color: selectedColor,
          weight: 4.5,
          opacity: 0.95,
        }).addTo(grup);
        mainLine
          .bindTooltip(
            garisUkur.label ??
              `${selectedOpt.tipe} · ${selectedOpt.jarakKm} km · ~${selectedOpt.durasiMenit} mnt`,
            {
              permanent: true,
              direction: "center",
              className: "garda-peta-tooltip",
            },
          )
          .openTooltip();
        boundsTarget = mainLine;
      } else {
        const line = L.polyline([garisUkur.asal, garisUkur.tujuan], {
          color: "#0284c7",
          weight: 3,
          dashArray: "6 6",
          opacity: 0.9,
        }).addTo(grup);
        if (garisUkur.label) {
          line
            .bindTooltip(garisUkur.label, {
              permanent: true,
              direction: "center",
              className: "garda-peta-tooltip",
            })
            .openTooltip();
        }
        boundsTarget = line;
      }
    }

    for (const g of gangguanJalan ?? []) {
      if (!g.aktif) continue;
      const isTotal = g.tingkat === "TUTUP_TOTAL";
      L.circle([g.lat, g.lng], {
        radius: Math.max(g.radiusM, 150),
        color: isTotal ? "#dc2626" : "#d97706",
        weight: 1.5,
        fillColor: isTotal ? "#ef4444" : "#f59e0b",
        fillOpacity: 0.16,
      }).addTo(grup);
      const pin = L.marker([g.lat, g.lng], {
        icon: L.divIcon({
          className: "garda-pin-wadah",
          html: `<span class="garda-pin-gangguan${isTotal ? "" : " garda-pin-gangguan-peringatan"}">!</span>`,
          iconSize: [20, 20],
          iconAnchor: [10, 10],
        }),
        zIndexOffset: 750,
      }).addTo(grup);
      pin.bindTooltip(
        `${g.judul} (${g.jenis.replaceAll("_", " ")} · ${isTotal ? "Tutup Total" : "Terhambat"})${g.ruasJalan ? ` — ${g.ruasJalan}` : ""}`,
        { direction: "top", offset: [0, -10], className: "garda-peta-tooltip" },
      );
    }

    if (boundsTarget) {
      peta.fitBounds(boundsTarget.getBounds(), { padding: [48, 48], maxZoom: 11, animate: geserHalus() });
    }
    return () => {
      grup.remove();
    };
  }, [peta, garisUkur, gangguanJalan]);

  // Pin titik yang sedang dicari (geolokasi atau klik peta).
  useEffect(() => {
    if (!peta || !titikCari) return;
    const pin = L.marker([titikCari.lat, titikCari.lng], {
      icon: IKON_CARI(),
      interactive: false,
      keyboard: false,
      zIndexOffset: 1000,
    }).addTo(peta);
    pin.getElement()?.setAttribute("data-titik-cari", "");
    return () => {
      pin.remove();
    };
  }, [peta, titikCari]);

  return (
    <div
      ref={pasangPeta}
      data-testid="peta-leaflet"
      className="garda-peta size-full"
    />
  );
}
