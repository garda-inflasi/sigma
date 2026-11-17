"use client";

import "mapbox-gl/dist/mapbox-gl.css";
import { useTheme } from "next-themes";
import { useEffect, useMemo, useState } from "react";
import Map, { Layer, Marker, NavigationControl, Popup, Source } from "react-map-gl/mapbox";
import { geometriKeFeatureCollection, type GeometriWilayah } from "@/lib/api/spasial";
import { mapConfig } from "@/lib/design/tokens";
import { formatPersen } from "@/lib/format";
import { MarkerPin, type MarkerWilayah } from "./marker-pin";

const WARNA_STATUS = {
  defisit: "#e5484d",
  waspada: "#f5a524",
  seimbang: "#f5a524",
  normal: "#2fa77a",
  surplus: "#2fa77a",
};

type GeoFeatureCollection = ReturnType<typeof geometriKeFeatureCollection>;

interface HoverWilayah {
  kode: string;
  nama: string;
  komoditas?: string;
  delta?: number;
  status?: string;
  lat: number;
  lng: number;
}

export default function MapboxRegionMap({
  token,
  markers,
  geometri,
  selected,
  onSelect,
  onKlikPeta,
}: {
  token: string;
  markers: MarkerWilayah[];
  geometri?: GeometriWilayah[];
  selected?: string;
  onSelect?: (kode: string) => void;
  onKlikPeta?: (lat: number, lng: number) => void;
}) {
  const { resolvedTheme } = useTheme();
  const warna = [
    "match",
    ["get", "status"],
    "defisit",
    WARNA_STATUS.defisit,
    "waspada",
    WARNA_STATUS.waspada,
    "seimbang",
    WARNA_STATUS.seimbang,
    "surplus",
    WARNA_STATUS.surplus,
    WARNA_STATUS.normal,
  ] as never;
  const [popup, setPopup] = useState<MarkerWilayah | null>(null);
  const [hover, setHover] = useState<HoverWilayah | null>(null);
  const [statisGeoJson, setStatisGeoJson] = useState<GeoFeatureCollection | null>(null);

  useEffect(() => {
    if (geometri && geometri.length > 0) return;
    const ctrl = new AbortController();
    fetch("/geojson/aceh-kabupaten.geojson", { signal: ctrl.signal })
      .then((r) => (r.ok ? (r.json() as Promise<GeoFeatureCollection>) : null))
      .then((fc) => {
        if (fc) setStatisGeoJson(fc);
      })
      .catch(() => {});
    return () => ctrl.abort();
  }, [geometri]);

  const markerByKode = useMemo(
    () => new globalThis.Map(markers.map((m) => [m.kode, m])),
    [markers],
  );

  const geojsonData = useMemo<GeoFeatureCollection | null>(() => {
    if (geometri && geometri.length > 0) {
      return geometriKeFeatureCollection(geometri, markers);
    }
    if (!statisGeoJson) return null;
    return {
      type: "FeatureCollection",
      features: statisGeoJson.features.map((f) => {
        const kode = String(f.properties?.kode ?? "");
        const m = markerByKode.get(kode);
        return {
          ...f,
          properties: {
            ...f.properties,
            kode,
            nama: String(f.properties?.nama ?? m?.nama ?? kode),
            status: m?.status ?? "normal",
            perkiraan: false,
            komoditas: m?.komoditas ?? "",
            delta: m?.delta ?? 0,
          },
        };
      }),
    };
  }, [geometri, markers, statisGeoJson, markerByKode]);

  const activePopup = popup ?? hover;

  return (
    <Map
      mapboxAccessToken={token}
      initialViewState={{ ...mapConfig.center, zoom: mapConfig.zoom }}
      maxBounds={mapConfig.bounds}
      mapStyle={resolvedTheme === "dark" ? mapConfig.styleDark : mapConfig.style}
      style={{ width: "100%", height: "100%" }}
      attributionControl={false}
      cooperativeGestures
      interactiveLayerIds={geojsonData ? ["wilayah-isian"] : undefined}
      onMouseMove={(e) => {
        const feat = e.features?.[0] as { properties?: Record<string, unknown> } | undefined;
        if (!feat) {
          setHover(null);
          return;
        }
        const kode = String(feat.properties?.kode ?? "");
        const m = markerByKode.get(kode);
        setHover({
          kode,
          nama: m?.nama ?? String(feat.properties?.nama ?? kode),
          komoditas: m?.komoditas,
          delta: m?.delta,
          status: m?.status,
          lat: e.lngLat.lat,
          lng: e.lngLat.lng,
        });
      }}
      onMouseLeave={() => setHover(null)}
      onClick={(e) => {
        const feat = e.features?.[0] as { properties?: Record<string, unknown> } | undefined;
        if (feat && onSelect) {
          const kode = String(feat.properties?.kode ?? "");
          if (kode) {
            onSelect(kode);
            return;
          }
        }
        onKlikPeta?.(e.lngLat.lat, e.lngLat.lng);
      }}
    >
      <NavigationControl position="top-right" showCompass={false} />
      {geojsonData && (
        <Source id="wilayah" type="geojson" data={geojsonData}>
          <Layer
            id="wilayah-isian"
            type="fill"
            paint={{
              "fill-color": warna,
              "fill-opacity": [
                "case",
                ["==", ["get", "kode"], hover?.kode ?? selected ?? ""],
                0.45,
                0.24,
              ],
            }}
          />
          <Layer
            id="wilayah-garis"
            type="line"
            paint={{
              "line-color": warna,
              "line-width": [
                "case",
                ["==", ["get", "kode"], hover?.kode ?? selected ?? ""],
                2,
                1,
              ],
              "line-opacity": 0.75,
            }}
          />
        </Source>
      )}
      {markers.map((m, i) => (
        <Marker key={m.kode} longitude={m.lng} latitude={m.lat} anchor="center">
          <MarkerPin
            m={m}
            index={i}
            selected={selected === m.kode}
            onClick={() => {
              setPopup(m);
              onSelect?.(m.kode);
            }}
          />
        </Marker>
      ))}
      {activePopup && (
        <Popup
          longitude={activePopup.lng}
          latitude={activePopup.lat}
          anchor="bottom"
          offset={14}
          onClose={() => {
            setPopup(null);
            setHover(null);
          }}
          closeButton={false}
          className="garda-popup"
        >
          <p className="font-display text-sm font-semibold">{activePopup.nama}</p>
          {activePopup.komoditas ? (
            <p className="text-xs text-muted-foreground">
              {activePopup.komoditas}{" "}
              {typeof activePopup.delta === "number" && (
                <span className="num font-semibold">
                  {formatPersen(activePopup.delta, { signed: true })}
                </span>
              )}
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">Status: {activePopup.status ?? "normal"}</p>
          )}
        </Popup>
      )}
    </Map>
  );
}
