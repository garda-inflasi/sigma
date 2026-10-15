import { Suspense } from "react";
import { KanbanBoard } from "@/components/intervensi/kanban-board";

export const metadata = { title: "Intervensi" };

export default function IntervensiPage() {
  return (
    <Suspense>
      <KanbanBoard />
    </Suspense>
  );
}
