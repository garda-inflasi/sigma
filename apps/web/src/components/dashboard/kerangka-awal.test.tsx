/**
 * Render pertama widget dashboard operasional (di server dan sebelum query selesai) harus berupa kerangka, bukan
 * panel galat: query yang belum berjalan tidak boleh terbaca "gagal" (kilatan galat sebelum data tiba).
 */
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";
import type { ComponentType } from "react";
import { renderToString } from "react-dom/server";
import { TrendChart } from "@/components/charts/trend-chart";
import { ValidationQueue } from "@/components/harga/validation-queue";
import { SessionProvider } from "@/lib/auth/session-context";
import { penggunaSesi } from "@/test/render";
import { FieldActivity } from "./field-activity";
import { CuacaWilayahCard, PembandingHargaCard } from "./hasil-ingestion";
import { KpiGrid } from "./kpi-grid";
import { OpsMap } from "./ops-map";
import { PriceHeatmap } from "./price-heatmap";

const WIDGET: Record<string, ComponentType> = {
  KpiGrid,
  PriceHeatmap,
  FieldActivity,
  OpsMap,
  TrendChart,
  ValidationQueue,
  PembandingHargaCard,
  CuacaWilayahCard,
};

const router = { push() {}, replace() {}, refresh() {}, back() {}, forward() {}, prefetch() {} };

describe("render pertama widget dashboard operasional", () => {
  for (const [nama, Widget] of Object.entries(WIDGET)) {
    it(`${nama}: kerangka memuat, tanpa panel galat`, () => {
      const html = renderToString(
        <QueryClientProvider client={new QueryClient()}>
          <SessionProvider user={penggunaSesi()}>
            <AppRouterContext.Provider value={router as never}>
              <Widget />
            </AppRouterContext.Provider>
          </SessionProvider>
        </QueryClientProvider>,
      );
      assert.match(html, /shimmer/);
      assert.doesNotMatch(html, /gagal dimuat|role="alert"/);
    });
  }
});
