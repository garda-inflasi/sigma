"use client";

import "mapbox-gl/dist/mapbox-gl.css";
import { useTheme } from "next-themes";
import { useState } from "react";
import Map, { Marker, NavigationControl, Popup } from "react-map-gl/mapbox";
import { mapConfig } from "@/lib/design/tokens";
import { formatPersen } from "@/lib/format";
import { MarkerPin, type MarkerWilayah } from "./marker-pin";

export default function MapboxRegionMap({ token, markers, selected, onSelect }: { token: string; markers: MarkerWilayah[]; selected?: string; onSelect?: (kode: string) => void }) {
  const { resolvedTheme } = useTheme();
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
