"use client";

import {
  flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable,
  type ColumnDef, type ColumnFiltersState, type Row, type RowSelectionState, type SortingState, type VisibilityState,
} from "@tanstack/react-table";
import { AnimatePresence, motion } from "motion/react";
import { ArrowDown, ArrowUp, ChevronLeft, ChevronRight, ChevronsUpDown, Inbox, Search } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { TableSkeleton } from "@/components/garda/shimmer";
import { toneClass, type Tone } from "@/lib/design/tokens";
import { fadeIn } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";
import { BulkActionBar, type BulkAction } from "./bulk-action-bar";

export interface Facet {
  columnId: string;
  label: string;
  options: { value: string; label: string }[];
}

interface DataTableProps<T> {
  data: T[] | undefined;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  columns: ColumnDef<T, any>[];
  getRowId: (row: T) => string;
  entityLabel: string;
  bulkActions?: BulkAction<T>[];
  isLoading?: boolean;
  searchPlaceholder?: string;
  facets?: Facet[];
  initialFilters?: ColumnFiltersState;
  initialSorting?: SortingState;
  toolbar?: ReactNode;
  pageSize?: number;
  rowTone?: (row: T) => Tone | undefined;
  onRowClick?: (row: T) => void;
  empty?: { title: string; description: string; action?: ReactNode };
  /** Kolom yang hanya dipakai untuk filter/facet, tidak ditampilkan */
  hiddenColumns?: string[];
  className?: string;
}

/**
 * Tabel data Garda — seluruh baris dapat dicentang untuk proses massal.
 * - Centang header: pilih semua baris di halaman (indeterminate bila sebagian)
 * - Banner "Pilih seluruh N data" untuk semua hasil filter lintas halaman
 * - Shift + klik: pilih rentang
 * - Esc: batalkan pilihan
 */
export function DataTable<T>({
  data, columns, getRowId, entityLabel, bulkActions = [], isLoading, searchPlaceholder = "Cari…", facets = [],
  initialFilters = [], initialSorting = [], toolbar, pageSize = 25, rowTone, onRowClick, empty, hiddenColumns = [], className,
}: DataTableProps<T>) {
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>(() => Object.fromEntries(hiddenColumns.map((c) => [c, false])));
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [sorting, setSorting] = useState<SortingState>(initialSorting);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>(initialFilters);
  const [globalFilter, setGlobalFilter] = useState("");
  const lastIndex = useRef<number | null>(null);

  const selectColumn = useMemo<ColumnDef<T>>(
    () => ({
      id: "_pilih",
      size: 44,
      enableSorting: false,
      header: ({ table }) => (
        <Checkbox
          aria-label="Pilih semua di halaman ini"
          checked={table.getIsAllPageRowsSelected() ? true : table.getIsSomePageRowsSelected() ? "indeterminate" : false}
          onCheckedChange={(v: boolean | "indeterminate") => table.toggleAllPageRowsSelected(v === true)}
          className="rounded-md"
        />
      ),
      cell: () => null, // dirender khusus di baris agar dapat menangkap shift-klik
    }),
    [],
  );

  const table = useReactTable({
    data: data ?? [],
    columns: useMemo(() => (bulkActions.length ? [selectColumn, ...columns] : columns), [bulkActions.length, selectColumn, columns]),
    getRowId: (r) => getRowId(r),
    state: { rowSelection, sorting, columnFilters, globalFilter, columnVisibility },
    onColumnVisibilityChange: setColumnVisibility,
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize } },
    autoResetPageIndex: false,
  });

  const pageRows = table.getRowModel().rows;
  const filteredRows = table.getFilteredRowModel().rows;
  const selectedRows = table.getSelectedRowModel().rows.map((r) => r.original);
  const clear = useCallback(() => setRowSelection({}), []);

  // Esc membatalkan pilihan
  useEffect(() => {
    if (!selectedRows.length) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && clear();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [selectedRows.length, clear]);

  // Pilihan dibersihkan dari ID yang sudah tidak ada (mis. setelah aksi menghapus baris)
  useEffect(() => {
    if (!data) return;
    const ids = new Set(data.map(getRowId));
    setRowSelection((s) => {
      const next = Object.fromEntries(Object.entries(s).filter(([k]) => ids.has(k)));
      return Object.keys(next).length === Object.keys(s).length ? s : next;
    });
  }, [data, getRowId]);

  function onRowCheck(row: Row<T>, index: number, shift: boolean) {
    if (shift && lastIndex.current !== null) {
      const [a, b] = [lastIndex.current, index].sort((x, y) => x - y);
      const target = !row.getIsSelected();
      setRowSelection((s) => {
        const next = { ...s };
        pageRows.slice(a, b + 1).forEach((r) => (target ? (next[r.id] = true) : delete next[r.id]));
        return next;
      });
    } else {
      row.toggleSelected();
    }
    lastIndex.current = index;
  }

  const allPageSelected = table.getIsAllPageRowsSelected();
  const allFilteredSelected = filteredRows.length > 0 && filteredRows.every((r) => rowSelection[r.id]);
  const { pageIndex, pageSize: ps } = table.getState().pagination;
  const from = filteredRows.length ? pageIndex * ps + 1 : 0;
  const to = Math.min(filteredRows.length, (pageIndex + 1) * ps);

  return (
    <div className={cn("glass overflow-hidden rounded-xl", className)}>
      {/* Toolbar */}
      <div className="flex flex-col gap-3 border-b border-border/70 p-4 lg:flex-row lg:items-center">
        <label className="relative flex-1 lg:max-w-sm">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <input
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            placeholder={searchPlaceholder}
            className="h-10 w-full rounded-md border border-input bg-white/70 pr-3 pl-9 text-sm outline-none transition-shadow placeholder:text-muted-foreground focus:shadow-glow-data dark:bg-white/5"
          />
        </label>
        <div className="flex flex-wrap items-center gap-2">
          {facets.map((f) => {
            const val = (table.getColumn(f.columnId)?.getFilterValue() as string) ?? "__semua";
            return (
              <Select key={f.columnId} value={val} onValueChange={(v: string) => table.getColumn(f.columnId)?.setFilterValue(v === "__semua" ? undefined : v)}>
                <SelectTrigger className="h-10 min-w-36 rounded-md bg-white/70 dark:bg-white/5">
                  <SelectValue placeholder={f.label} />
                </SelectTrigger>
                <SelectContent className="rounded-xl">
                  <SelectItem value="__semua">{f.label}: semua</SelectItem>
                  {f.options.map((o) => (
                    <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            );
          })}
          {toolbar}
        </div>
      </div>

      {/* Banner pilih-semua lintas halaman */}
      <AnimatePresence initial={false}>
        {allPageSelected && filteredRows.length > pageRows.length && (
          <motion.div variants={fadeIn} initial="hidden" animate="show" exit="exit" className="flex flex-wrap items-center justify-center gap-x-2 gap-y-1 border-b border-border/70 bg-data-50/70 px-4 py-2 text-sm text-data-800 dark:bg-data-500/10 dark:text-data-200">
            {allFilteredSelected ? (
              <>
                Seluruh <span className="num font-semibold">{filteredRows.length.toLocaleString("id-ID")}</span> {entityLabel} terpilih.
                <button onClick={clear} className="font-semibold underline-offset-4 hover:underline">Batalkan pilihan</button>
              </>
            ) : (
              <>
                <span className="num font-semibold">{pageRows.length}</span> {entityLabel} di halaman ini terpilih.
                <button
                  onClick={() => setRowSelection(Object.fromEntries(filteredRows.map((r) => [r.id, true])))}
                  className="font-semibold underline-offset-4 hover:underline"
                >
                  Pilih seluruh {filteredRows.length.toLocaleString("id-ID")} {entityLabel}
                </button>
              </>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Tabel */}
      <div className="scrollbar-thin overflow-x-auto">
        {isLoading ? (
          <TableSkeleton cols={Math.min(6, columns.length)} />
        ) : (
          <table className="w-full min-w-[760px] border-separate border-spacing-0 text-sm">
            <thead className="sticky top-0 z-10 bg-white/80 backdrop-blur-xl dark:bg-navy-950/80">
              {table.getHeaderGroups().map((hg) => (
                <tr key={hg.id}>
                  {hg.headers.map((h) => {
                    const sortable = h.column.getCanSort();
                    const dir = h.column.getIsSorted();
                    return (
                      <th
                        key={h.id}
                        style={{ width: h.getSize() !== 150 ? h.getSize() : undefined }}
                        className="h-11 border-b border-border/70 px-3 text-left text-xs font-medium whitespace-nowrap text-muted-foreground first:pl-5"
                      >
                        {h.isPlaceholder ? null : sortable ? (
                          <button onClick={h.column.getToggleSortingHandler()} className="inline-flex items-center gap-1 hover:text-foreground">
                            {flexRender(h.column.columnDef.header, h.getContext())}
                            {dir === "asc" ? <ArrowUp className="size-3" /> : dir === "desc" ? <ArrowDown className="size-3" /> : <ChevronsUpDown className="size-3 opacity-40" />}
                          </button>
                        ) : (
                          flexRender(h.column.columnDef.header, h.getContext())
                        )}
                      </th>
                    );
                  })}
                </tr>
              ))}
            </thead>
            <tbody>
              {pageRows.map((row, i) => {
                const selected = row.getIsSelected();
                const tone = rowTone?.(row.original);
                return (
                  <tr
                    key={row.id}
                    data-state={selected ? "selected" : undefined}
                    onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                    className={cn(
                      "group transition-colors",
                      selected ? "bg-data-50/80 dark:bg-data-500/10" : "hover:bg-white/60 dark:hover:bg-white/[0.03]",
                      onRowClick && "cursor-pointer",
                    )}
                  >
                    {row.getVisibleCells().map((cell, ci) => (
                      <td key={cell.id} className={cn("relative h-14 border-b border-border/50 px-3 align-middle first:pl-5", ci === 0 && "w-11")}>
                        {ci === 0 && (selected || tone) && (
                          <span className={cn("absolute inset-y-2 left-0 w-[3px] rounded-r-full", selected ? "bg-data-500" : tone && toneClass[tone].bg)} />
                        )}
                        {cell.column.id === "_pilih" ? (
                          <span onClick={(e) => { e.stopPropagation(); onRowCheck(row, i, e.shiftKey); }} className="inline-flex">
                            <Checkbox checked={selected} aria-label="Pilih baris" className="rounded-md" />
                          </span>
                        ) : (
                          flexRender(cell.column.columnDef.cell, cell.getContext())
                        )}
                      </td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
        {!isLoading && pageRows.length === 0 && (
          <div className="grid place-items-center gap-2 px-6 py-16 text-center">
            <span className="grid size-12 place-items-center rounded-xl bg-mist-100 text-muted-foreground dark:bg-white/5"><Inbox className="size-5" /></span>
            <p className="font-display font-semibold">{empty?.title ?? `Tidak ada ${entityLabel} yang cocok`}</p>
            <p className="max-w-sm text-sm text-muted-foreground">{empty?.description ?? "Ubah kata kunci atau filter untuk melihat data lain."}</p>
            {empty?.action}
          </div>
        )}
      </div>

      {/* Paginasi */}
      <div className="flex flex-col items-center justify-between gap-3 px-5 py-3 text-sm text-muted-foreground sm:flex-row">
        <span>
          <span className="num text-foreground">{from.toLocaleString("id-ID")}–{to.toLocaleString("id-ID")}</span> dari{" "}
          <span className="num text-foreground">{filteredRows.length.toLocaleString("id-ID")}</span> {entityLabel}
          {selectedRows.length > 0 && (
            <span className="ml-2 rounded-full bg-data-50 px-2 py-0.5 text-xs text-data-700 dark:bg-data-500/15 dark:text-data-300">
              <span className="num">{selectedRows.length.toLocaleString("id-ID")}</span> terpilih
            </span>
          )}
        </span>
        <div className="flex items-center gap-2">
          <Select value={String(ps)} onValueChange={(v: string) => table.setPageSize(Number(v))}>
            <SelectTrigger className="h-8 w-[110px] rounded-md"><SelectValue /></SelectTrigger>
            <SelectContent>{[10, 25, 50, 100].map((n) => <SelectItem key={n} value={String(n)}>{n} / halaman</SelectItem>)}</SelectContent>
          </Select>
          <button onClick={() => table.previousPage()} disabled={!table.getCanPreviousPage()} className="grid size-8 place-items-center rounded-md hover:bg-secondary disabled:opacity-40" aria-label="Halaman sebelumnya"><ChevronLeft className="size-4" /></button>
          <span className="num min-w-14 text-center text-foreground">{pageIndex + 1} / {Math.max(1, table.getPageCount())}</span>
          <button onClick={() => table.nextPage()} disabled={!table.getCanNextPage()} className="grid size-8 place-items-center rounded-md hover:bg-secondary disabled:opacity-40" aria-label="Halaman berikutnya"><ChevronRight className="size-4" /></button>
        </div>
      </div>

      {bulkActions.length > 0 && <BulkActionBar rows={selectedRows} actions={bulkActions} entityLabel={entityLabel} onClear={clear} />}
    </div>
  );
}
