import { Suspense } from "react";
import { AuditPage } from "@/components/audit/audit-page";

export const metadata = { title: "Audit trail" };

export default function Page() {
  return (
    <Suspense>
      <AuditPage />
    </Suspense>
  );
}
