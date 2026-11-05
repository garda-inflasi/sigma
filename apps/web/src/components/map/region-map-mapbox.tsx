"use client";

import "mapbox-gl/dist/mapbox-gl.css";
import { useTheme } from "next-themes";
import { useState } from "react";
import Map, { Layer, Marker, NavigationControl, Popup, Source } from "react-map-gl/mapbox";
import { geometriKeFeatureCollection, type GeometriWilayah } from "@/lib/api/spasial";
import { mapConfig } from "@/lib/design/tokens";
import { formatPersen } from "@/lib/format";
import { MarkerPin, type MarkerWilayah } from "./marker-pin";

const WARNA_STATUS = { defisit: "#e5484d", waspada: "#f5a524", normal: "#2fa77a", surplus: "#3b82f6" };

export default function MapboxRegionMap({ token, markers, geometri, selected, onSelect }: { token: string; markers: MarkerWilayah[]; geometri?: GeometriWilayah[]; selected?: string; onSelect?: (kode: string) => void }) {
  const { resolvedTheme } = useTheme();
  const warna = ["match", ["get", "status"], "defisit", WARNA_STATUS.defisit, "waspada", WARNA_STATUS.waspada, "surplus", WARNA_STATUS.surplus, WARNA_STATUS.normal] as never;
  const [popup, setPopup] = useState<MarkerWilayah | null>(null);
  return (
    <Map
      mapboxAccessToken={token}
      initialViewState={{ ...mapConfig.center, zoom: mapConfig.zoom }}
      maxBounds={mapConfig.bounds}
      mapStyle={resolvedTheme === "dark" ? mapConfig.styleDark : mapConfig.style}
      style={{ width: "100%", height: "100%" }}
      attributionControl={false}
      cooperativeGestures
    >
      <NavigationControl position="top-right" showCompass={false} />
      {geometri && geometri.length > 0 && (
        <Source id="wilayah" type="geojson" data={geometriKeFeatureCollection(geometri, markers)}>
          <Layer id="wilayah-isian" type="fill" paint={{ "fill-color": warna, "fill-opacity": 0.18 }} />
          <Layer id="wilayah-garis" type="line" paint={{ "line-color": warna, "line-width": 1, "line-opacity": 0.6 }} />
        </Source>
      )}
      {markers.map((m, i) => (
        <Marker key={m.kode} longitude={m.lng} latitude={m.lat} anchor="center">
          <MarkerPin m={m} index={i} selected={selected === m.kode} onClick={() => { setPopup(m); onSelect?.(m.kode); }} />
        </Marker>
      ))}
      {popup && (
        <Popup longitude={popup.lng} latitude={popup.lat} anchor="bottom" offset={14} onClose={() => setPopup(null)} closeButton={false} className="garda-popup">
          <p className="font-display text-sm font-semibold">{popup.nama}</p>
          <p className="text-xs text-muted-foreground">{popup.komoditas} <span className="num font-semibold">{formatPersen(popup.delta, { signed: true })}</span></p>
        </Popup>
      )}
    </Map>
  );
}
