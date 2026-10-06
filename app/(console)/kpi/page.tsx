import { Suspense } from "react";
import { KpiPage } from "@/components/kpi/kpi-page";

export const metadata = { title: "Indikator kinerja" };

export default function Page() {
  return (
    <Suspense>
      <KpiPage />
    </Suspense>
  );
}
