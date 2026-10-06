import { Suspense } from "react";
import { EwsPage } from "@/components/ews/ews-page";

export const metadata = { title: "Peringatan dini" };

export default function Page() {
  return (
    <Suspense>
      <EwsPage />
    </Suspense>
  );
}
