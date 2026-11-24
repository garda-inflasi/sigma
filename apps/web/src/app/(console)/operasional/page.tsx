import { Suspense } from "react";
import { TampilanOperasional } from "@/components/dashboard/tampilan-operasional";
import { HalamanPas, IsiPas } from "@/components/garda/halaman-pas";
import { Shimmer } from "@/components/garda/shimmer";
import { getSession } from "@/lib/auth/session";
import { can } from "@/lib/rbac/permissions";

export const metadata = { title: "Dashboard operasional" };

function OperasionalSkeleton() {
  return (
    <HalamanPas>
      <Shimmer className="h-12 w-1/3 shrink-0 rounded-lg" />
      <div className="grid shrink-0 grid-cols-2 gap-2 lg:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Shimmer key={i} className="h-[4.75rem] rounded-lg" />
        ))}
      </div>
      <IsiPas className="lg:grid lg:grid-cols-12">
        <Shimmer className="h-72 rounded-lg lg:col-span-8 lg:h-auto" />
        <Shimmer className="h-72 rounded-lg lg:col-span-4 lg:h-auto" />
      </IsiPas>
    </HalamanPas>
  );
}

export default async function OperasionalPage() {
  const user = await getSession();

  return (
    <Suspense fallback={<OperasionalSkeleton />}>
      <TampilanOperasional
        lihatPeta={can(user, "spasial.lihat")}
        lihatEws={can(user, "ews.lihat")}
        lihatHarga={can(user, "harga.lihat")}
        bisaValidasi={can(user, "harga.validasi")}
        bisaInput={can(user, "harga.input")}
      />
    </Suspense>
  );
}
