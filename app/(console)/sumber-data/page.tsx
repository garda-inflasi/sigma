import { Suspense } from "react";
import { SumberPage } from "@/components/sumber/sumber-page";

export const metadata = { title: "Sumber data & API" };

export default function Page() {
  return (
    <Suspense>
      <SumberPage />
    </Suspense>
  );
}
