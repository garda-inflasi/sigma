import { Suspense } from "react";
import { ImpersonasiPage } from "@/components/impersonasi/impersonasi-page";

export const metadata = { title: "Impersonasi" };

export default function Page() {
  return (
    <Suspense>
      <ImpersonasiPage />
    </Suspense>
  );
}
