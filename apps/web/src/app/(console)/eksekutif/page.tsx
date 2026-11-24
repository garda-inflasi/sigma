import { Suspense } from "react";
import { ExecutiveDashboard } from "@/components/eksekutif/executive-dashboard";
import { HalamanPas, IsiPas } from "@/components/garda/halaman-pas";
import { Shimmer } from "@/components/garda/shimmer";

export const metadata = { title: "Dashboard eksekutif" };

function EksekutifSkeleton() {
  return (
    <HalamanPas>
      <Shimmer className="h-12 w-1/3 shrink-0 rounded-lg" />
      <Shimmer className="h-[4.75rem] w-full shrink-0 rounded-lg" />
      <IsiPas className="lg:grid lg:grid-cols-12">
        <Shimmer className="h-72 rounded-lg lg:col-span-8 lg:h-auto" />
        <Shimmer className="h-72 rounded-lg lg:col-span-4 lg:h-auto" />
      </IsiPas>
    </HalamanPas>
  );
}

export default function EksekutifPage() {
  return (
    <Suspense fallback={<EksekutifSkeleton />}>
      <ExecutiveDashboard />
    </Suspense>
  );
}
