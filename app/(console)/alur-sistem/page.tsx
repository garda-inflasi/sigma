import { Suspense } from "react";
import { AlurPage } from "@/components/alur/alur-page";

export const metadata = { title: "Alur sistem & data" };

export default function Page() {
  return (
    <Suspense>
      <AlurPage />
    </Suspense>
  );
}
