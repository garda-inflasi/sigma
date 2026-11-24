import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { MarkerWilayah } from "@/lib/peta/aceh";
import {
  geometriKeFeatureCollection,
  toGeometri,
  type GeometriResponse,
} from "./spasial";

const respons: GeometriResponse = {
  type: "FeatureCollection",
  features: [
    {
      id: "1171",
      properties: {
        kode: "1171",
        nama: "KOTA BANDA ACEH",
        level: "kota",
        sumber: "DEMO",
        perkiraan: true,
        keterangan: "",
      },
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [95.2, 5.5],
            [95.4, 5.5],
            [95.4, 5.6],
            [95.2, 5.5],
          ],
          [
            [95.25, 5.51],
            [95.3, 5.51],
            [95.3, 5.53],
            [95.25, 5.51],
          ],
        ],
      },
    },
    {
      id: "1172",
      properties: {
        kode: "1172",
        nama: "KOTA SABANG",
        level: "kota",
        sumber: "RESMI",
        perkiraan: false,
        keterangan: "",
      },
      geometry: {
        type: "MultiPolygon",
        coordinates: [
          [
            [
              [95.3, 5.8],
              [95.4, 5.8],
              [95.4, 5.9],
              [95.3, 5.8],
            ],
          ],
          [
            [
              [95.5, 5.8],
              [95.6, 5.8],
              [95.6, 5.9],
              [95.5, 5.8],
            ],
          ],
        ],
      },
    },
  ],
};

describe("toGeometri", () => {
  it("memformat nama dan menandai poligon perkiraan", () => {
    const [bandaAceh, sabang] = toGeometri(respons);

    assert.equal(bandaAceh.nama, "Kota Banda Aceh");
    assert.equal(bandaAceh.perkiraan, true);
    assert.equal(sabang.perkiraan, false);
  });
});

describe("geometriKeFeatureCollection", () => {
  const marker = (
    kode: string,
    status: MarkerWilayah["status"],
  ): MarkerWilayah => ({
    kode,
    nama: kode,
    lat: 0,
    lng: 0,
    delta: 0,
    komoditas: "-",
    status,
    punyaPasar: true,
  });

  it("membawa status marker ke properti fitur; wilayah tanpa marker dianggap normal", () => {
    const fc = geometriKeFeatureCollection(toGeometri(respons), [
      marker("1171", "defisit"),
    ]);

    assert.equal(fc.type, "FeatureCollection");
    assert.deepEqual(
      fc.features.map((f) => [f.id, f.properties.status]),
      [
        ["1171", "defisit"],
        ["1172", "normal"],
      ],
    );
    assert.equal(fc.features[0].geometry.type, "Polygon");
  });
});
