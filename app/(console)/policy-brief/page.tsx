import { Suspense } from "react";
import { BriefPage } from "@/components/brief/brief-page";

export const metadata = { title: "Policy brief" };

export default function Page() {
  return (
    <Suspense>
      <BriefPage />
    </Suspense>
  );
}
