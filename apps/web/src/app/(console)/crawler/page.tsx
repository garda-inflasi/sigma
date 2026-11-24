import { Suspense } from "react";
import { CrawlerPage } from "@/components/crawler/crawler-page";

export const metadata = { title: "Pipeline & Crawler" };

export default function Page() {
  return (
    <Suspense>
      <CrawlerPage />
    </Suspense>
  );
}
