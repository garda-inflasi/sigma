"use client";

import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type ColumnFiltersState,
  type Row,
  type RowData,
  type RowSelectionState,
  type SortingState,
  type VisibilityState,
} from "@tanstack/react-table";
import { AnimatePresence, motion } from "motion/react";
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronsUpDown,
  Search,
  X,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { EmptyState } from "@/components/garda/empty-state";
import { TableSkeleton } from "@/components/garda/shimmer";
import { toneClass, type Tone } from "@/lib/design/tokens";
import { fadeIn } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";
import { BulkActionBar, type BulkAction } from "./bulk-action-bar";
import {
  catatanPilihanHalaman,
  jumlahHalamanTabel,
  rentangBaris,
  type ServerTable,
} from "./server-mode";

declare module "@tanstack/react-table" {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    /** Kolom menempel di tepi kanan saat tabel digulir mendatar (kolom aksi). */
    sematKanan?: boolean;
  }
}

/** Teks sel yang dipotong dengan elipsis; nilai lengkapnya tetap terbaca lewat `title`. */
export function SelTeks({
  children,
  className,
}: {
  children: string | null | undefined;
  className?: string;
}) {
  return (
    <span
      title={children ?? undefined}
      className={cn("block max-w-[16rem] truncate", className)}
    >
      {children}
    </span>
  );
}

export interface Facet {
  columnId: string;
  label: string;
  options: { value: string; label: string }[];
}

/**
 * Pengelompokan baris dengan header lipat (accordion): mis. entri harga per status. Urutan grup mengikuti `order`
 * (kunci lain di akhir); baris di dalam grup tetap urut sesuai pengurutan tabel. Pilih-semua dan aksi massal tetap
 * bekerja lintas grup, dan baris di grup terlipat tetap terhitung terpilih.
 */
export interface GroupBy<T> {
  key: (row: T) => string;
  order?: string[];
  label: (key: string) => string;
  tone?: (key: string) => Tone | undefined;
  /** Kunci yang awalnya terlipat. */
  collapsed?: string[];
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
  /**
   * Urutan yang selalu berlaku di depan urutan pilihan pengguna, mis. menahan baris prioritas di puncak lintas halaman.
   * Kolomnya tidak bisa diganti lewat header (mode klien saja).
   */
  urutanTetap?: SortingState;
  toolbar?: ReactNode;
  pageSize?: number;
  rowTone?: (row: T) => Tone | undefined;
  groupBy?: GroupBy<T>;
  onRowClick?: (row: T) => void;
  empty?: { title: string; description: string; action?: ReactNode };
  /** Kolom yang hanya dipakai untuk filter/facet, tidak ditampilkan */
  hiddenColumns?: string[];
  /**
   * Mode server: `data` adalah SATU halaman dari server, dan pencarian, urutan, filter, serta paginasi dijalankan
   * di sana (lihat ServerTable). Tanpa ini tabel bekerja di klien atas seluruh `data`, seperti biasa.
   */
  server?: ServerTable;
  /** Baris tambahan di bawah toolbar, mis. kontrol filter khusus halaman (mode server). */
  filterBar?: ReactNode;
  /** Sembunyikan kotak cari (mis. daftar server yang tidak mendukung pencarian). */
  tanpaCari?: boolean;
  /** Desktop: tabel mengisi tinggi wadah, body menggulir sendiri, dan paginasi tetap terlihat. */
  isiPenuh?: boolean;
  className?: string;
}

/** Urutan tetap di depan, lalu urutan pilihan pengguna tanpa kolom yang sama. */
function gabungUrutan(tetap: SortingState | undefined, pilihan: SortingState): SortingState {
  if (!tetap?.length) return pilihan;
  return [...tetap, ...pilihan.filter((u) => !tetap.some((t) => t.id === u.id))];
}

/**
 * Tabel data Garda — seluruh baris dapat dicentang untuk proses massal.
 * - Centang header: pilih semua baris di halaman (indeterminate bila sebagian)
 * - Banner "Pilih seluruh N data" untuk semua hasil filter lintas halaman
 * - Shift + klik: pilih rentang
 * - Esc: batalkan pilihan
 */
export function DataTable<T>({
  data,
  columns,
  getRowId,
  entityLabel,
  bulkActions = [],
  isLoading,
  searchPlaceholder = "Cari…",
  facets = [],
  initialFilters = [],
  initialSorting = [],
  urutanTetap,
  toolbar,
  pageSize = 25,
  rowTone,
  groupBy,
  onRowClick,
  empty,
  hiddenColumns = [],
  server,
  filterBar,
  tanpaCari = false,
  isiPenuh = false,
  className,
}: DataTableProps<T>) {
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>(
    () => Object.fromEntries(hiddenColumns.map((c) => [c, false])),
  );
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [sorting, setSorting] = useState<SortingState>(initialSorting);
  const [columnFilters, setColumnFilters] =
    useState<ColumnFiltersState>(initialFilters);
  const [globalFilter, setGlobalFilter] = useState("");
  const lastIndex = useRef<number | null>(null);
  const gulirRef = useRef<HTMLDivElement>(null);
  const [tepi, setTepi] = useState({ kiri: false, kanan: false });

  // Indikator gulir mendatar: ResizeObserver juga memanggil `ukur` sekali saat mulai mengamati.
  useEffect(() => {
    const el = gulirRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ukur = () => {
      const kiri = el.scrollLeft > 1;
      const kanan = el.scrollLeft + el.clientWidth < el.scrollWidth - 1;
      setTepi((t) =>
        t.kiri === kiri && t.kanan === kanan ? t : { kiri, kanan },
      );
    };
    const pengamat = new ResizeObserver(ukur);
    pengamat.observe(el);
    if (el.firstElementChild) pengamat.observe(el.firstElementChild);
    el.addEventListener("scroll", ukur, { passive: true });
    return () => {
      pengamat.disconnect();
      el.removeEventListener("scroll", ukur);
    };
  }, [isLoading]);
  const [terlipat, setTerlipat] = useState<Set<string>>(
    () => new Set(groupBy?.collapsed ?? []),
  );

  // Mode server: kotak cari memegang draft ketikan; kata kunci dikirim ke server setelah jeda ketik atau Enter.
  const adaServer = !!server;
  const searchServer = server?.search;
  const jedaCari = server?.searchDelayMs ?? 300;
  const [draftCari, setDraftCari] = useState(searchServer ?? "");
  const cariDikirim = useRef(searchServer ?? "");
  const kirimCari = useRef(server?.onSearchChange);
  useEffect(() => {
    kirimCari.current = server?.onSearchChange;
  });
  // Kata kunci berubah dari luar (tombol kembali, tautan): kotak cari ikut.
  useEffect(() => {
    if (searchServer === undefined || searchServer === cariDikirim.current)
      return;
    cariDikirim.current = searchServer;
    setDraftCari(searchServer);
  }, [searchServer]);
  useEffect(() => {
    if (!adaServer) return;
    const kata = draftCari.trim();
    if (kata === searchServer) return;
    const timer = setTimeout(() => {
      cariDikirim.current = kata;
      kirimCari.current?.(kata);
    }, jedaCari);
    return () => clearTimeout(timer);
  }, [adaServer, draftCari, searchServer, jedaCari]);

  // Mode server: pilihan baris hanya berlaku untuk halaman yang tampil, jadi dibersihkan begitu kueri (halaman,
  // urutan, pencarian, filter) berubah. Aksi massal tidak boleh mengenai baris yang sudah tidak terlihat.
  const [kunciTampil, setKunciTampil] = useState(server?.kunciKueri);
  if (server && server.kunciKueri !== kunciTampil) {
    setKunciTampil(server.kunciKueri);
    setRowSelection({});
  }

  const selectColumn = useMemo<ColumnDef<T>>(
    () => ({
      id: "_pilih",
      size: 44,
      enableSorting: false,
      header: ({ table }) => (
        <Checkbox
          aria-label="Pilih semua di halaman ini"
          checked={
            table.getIsAllPageRowsSelected()
              ? true
              : table.getIsSomePageRowsSelected()
                ? "indeterminate"
                : false
          }
          onCheckedChange={(v: boolean | "indeterminate") =>
            table.toggleAllPageRowsSelected(v === true)
          }
          className="rounded-md"
        />
      ),
      cell: () => null, // dirender khusus di baris agar dapat menangkap shift-klik
    }),
    [],
  );

  const urutanEfektif = useMemo(
    () => gabungUrutan(urutanTetap, sorting),
    [urutanTetap, sorting],
  );

  const table = useReactTable({
    data: data ?? [],
    columns: useMemo(
      () => (bulkActions.length ? [selectColumn, ...columns] : columns),
      [bulkActions.length, selectColumn, columns],
    ),
    getRowId: (r) => getRowId(r),
    state: {
      rowSelection,
      sorting: server ? server.sorting : urutanEfektif,
      columnFilters,
      globalFilter: server ? "" : globalFilter,
      columnVisibility,
      ...(server
        ? {
            pagination: {
              pageIndex: server.page - 1,
              pageSize: server.pageSize,
            },
          }
        : {}),
    },
    manualPagination: adaServer,
    manualSorting: adaServer,
    manualFiltering: adaServer,
    rowCount: server?.total,
    onColumnVisibilityChange: setColumnVisibility,
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: (ubah) =>
      setSorting((lama) => {
        const baru = typeof ubah === "function" ? ubah(gabungUrutan(urutanTetap, lama)) : ubah;
        return baru.filter((u) => !urutanTetap?.some((t) => t.id === u.id));
      }),
    onColumnFiltersChange: setColumnFilters,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize } },
    autoResetPageIndex: false,
  });

  const barisHalaman = table.getRowModel().rows;
  // Dengan groupBy, baris disusun per grup (urutan grup tetap, urutan di dalam grup tetap) agar indeks
  // shift-klik sama dengan urutan yang terlihat.
  const kelompok = useMemo(() => {
    if (!groupBy) return null;
    const peta = new Map<string, Row<T>[]>();
    for (const r of barisHalaman) {
      const k = groupBy.key(r.original);
      peta.set(k, [...(peta.get(k) ?? []), r]);
    }
    const urut = groupBy.order ?? [];
    const kunci = [...peta.keys()].sort((a, b) => {
      const ia = urut.indexOf(a),
        ib = urut.indexOf(b);
      return (ia < 0 ? urut.length : ia) - (ib < 0 ? urut.length : ib);
    });
    return kunci.map((k) => ({ key: k, rows: peta.get(k)! }));
  }, [groupBy, barisHalaman]);
  const pageRows = useMemo(
    () => (kelompok ? kelompok.flatMap((g) => g.rows) : barisHalaman),
    [kelompok, barisHalaman],
  );
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
      const next = Object.fromEntries(
        Object.entries(s).filter(([k]) => ids.has(k)),
      );
      return Object.keys(next).length === Object.keys(s).length ? s : next;
    });
  }, [data, getRowId]);

  function onRowCheck(row: Row<T>, index: number, shift: boolean) {
    if (shift && lastIndex.current !== null) {
      const [a, b] = [lastIndex.current, index].sort((x, y) => x - y);
      const target = !row.getIsSelected();
      setRowSelection((s) => {
        const next = { ...s };
        pageRows
          .slice(a, b + 1)
          .forEach((r) => (target ? (next[r.id] = true) : delete next[r.id]));
        return next;
      });
    } else {
      row.toggleSelected();
    }
    lastIndex.current = index;
  }

  const allPageSelected = table.getIsAllPageRowsSelected();
  const allFilteredSelected =
    filteredRows.length > 0 && filteredRows.every((r) => rowSelection[r.id]);
  const { pageIndex, pageSize: ps } = table.getState().pagination;
  // Hasil menyusut (pencarian, filter, baris terhapus): jangan tertinggal di halaman yang sudah tidak ada.
  const halamanTerakhir = Math.max(0, table.getPageCount() - 1);
  useEffect(() => {
    if (!adaServer && pageIndex > halamanTerakhir) table.setPageIndex(halamanTerakhir);
  }, [adaServer, pageIndex, halamanTerakhir, table]);
  const totalBaris = server ? server.total : filteredRows.length;
  const { from, to } = server
    ? rentangBaris(server.total, server.page, server.pageSize)
    : {
        from: filteredRows.length ? pageIndex * ps + 1 : 0,
        to: Math.min(filteredRows.length, (pageIndex + 1) * ps),
      };
  const jumlahHal = server
    ? jumlahHalamanTabel(server.total, server.pageSize)
    : Math.max(1, table.getPageCount());
  const jumlahFilter = columnFilters.length + (globalFilter.trim() ? 1 : 0);
  const catatanServer = server
    ? catatanPilihanHalaman(
        selectedRows.length,
        pageRows.length,
        server.total,
        entityLabel,
      )
    : null;

  return (
    <div
      className={cn(
        "glass min-w-0 overflow-hidden rounded-lg",
        isiPenuh &&
          "flex w-full flex-col lg:max-h-full lg:min-h-0 lg:self-start",
        className,
      )}
    >
      {/* Toolbar 1 baris: kiri = Cari + Facet/Filter + filterBar + Reset; kanan = Aksi sekunder */}
      <div
        data-slot="datatable-toolbar"
        className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-border px-3 py-2"
      >
        <div
          data-slot="datatable-filters"
          className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5"
        >
          <label
            className={cn(
              "relative w-full min-w-0 shrink-0",
              facets.length > 0 || filterBar
                ? "sm:w-48 lg:w-52"
                : "sm:w-56 lg:w-64",
              tanpaCari && "hidden",
            )}
          >
            <Search
              className="pointer-events-none absolute top-1/2 left-2.5 z-[1] size-3.5 -translate-y-1/2 text-muted-foreground"
              strokeWidth={1.75}
            />
            <Input
              value={server ? draftCari : globalFilter}
              onChange={(e) =>
                server
                  ? setDraftCari(e.target.value)
                  : setGlobalFilter(e.target.value)
              }
              onKeyDown={(e) => {
                // Enter mengirim kata kunci tanpa menunggu jeda ketik.
                if (!server || e.key !== "Enter") return;
                const kata = draftCari.trim();
                if (kata !== server.search) {
                  cariDikirim.current = kata;
                  server.onSearchChange(kata);
                }
              }}
              placeholder={searchPlaceholder}
              aria-label={searchPlaceholder}
              className="h-8 pr-2.5 pl-8 md:text-[13px]"
            />
          </label>
          {facets.map((f) => {
            const val =
              (table.getColumn(f.columnId)?.getFilterValue() as string) ??
              "__semua";
            return (
              <Select
                key={f.columnId}
                value={val}
                onValueChange={(v: string) =>
                  table
                    .getColumn(f.columnId)
                    ?.setFilterValue(v === "__semua" ? undefined : v)
                }
              >
                <SelectTrigger
                  size="sm"
                  className="w-full min-w-0 text-[13px] sm:w-auto sm:min-w-28 sm:max-w-44 md:text-[13px]"
                >
                  <SelectValue placeholder={f.label} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__semua">{f.label}: semua</SelectItem>
                  {f.options.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            );
          })}
          {filterBar}
          {!server && jumlahFilter > 0 && (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 px-2.5 text-[12.5px]"
              onClick={() => {
                setColumnFilters([]);
                setGlobalFilter("");
                table.setPageIndex(0);
              }}
            >
              <X /> Reset filter ({jumlahFilter})
            </Button>
          )}
        </div>
        {toolbar && (
          <div
            data-slot="datatable-actions"
            className="flex min-w-0 shrink-0 flex-wrap items-center gap-1.5 sm:ml-auto sm:justify-end"
          >
            {toolbar}
          </div>
        )}
      </div>

      {/* Banner pilih-semua lintas halaman */}
      <AnimatePresence initial={false}>
        {allPageSelected && filteredRows.length > pageRows.length && (
          <motion.div
            variants={fadeIn}
            initial="hidden"
            animate="show"
            exit="exit"
            className="flex flex-wrap items-center justify-center gap-x-2 gap-y-1 border-b border-border bg-data-50 px-4 py-2 text-[13px] text-data-800 dark:bg-data-500/10 dark:text-data-200"
          >
            {allFilteredSelected ? (
              <>
                Seluruh{" "}
                <span className="num font-semibold">
                  {filteredRows.length.toLocaleString("id-ID")}
                </span>{" "}
                {entityLabel} terpilih.
                <button
                  type="button"
                  onClick={clear}
                  className="font-semibold underline-offset-4 hover:underline"
                >
                  Batalkan pilihan
                </button>
              </>
            ) : (
              <>
                <span className="num font-semibold">{pageRows.length}</span>{" "}
                {entityLabel} di halaman ini terpilih.
                <button
                  type="button"
                  onClick={() =>
                    setRowSelection(
                      Object.fromEntries(filteredRows.map((r) => [r.id, true])),
                    )
                  }
                  className="font-semibold underline-offset-4 hover:underline"
                >
                  Pilih seluruh {filteredRows.length.toLocaleString("id-ID")}{" "}
                  {entityLabel}
                </button>
              </>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Mode server: pilihan tidak pernah lintas halaman; jelaskan agar tidak ambigu */}
      {catatanServer && (
        <div
          role="status"
          className="flex flex-wrap items-center justify-center gap-x-2 gap-y-1 border-b border-border bg-data-50 px-4 py-2 text-[13px] text-data-800 dark:bg-data-500/10 dark:text-data-200"
        >
          {catatanServer}
          <button
            type="button"
            onClick={clear}
            className="font-semibold underline-offset-4 hover:underline"
          >
            Batalkan pilihan
          </button>
        </div>
      )}

      {/* Tabel */}
      <div className={cn("relative min-w-0", isiPenuh && "lg:flex lg:min-h-0 lg:flex-col")}>
        <div
          ref={gulirRef}
          data-lenis-prevent
          aria-busy={server?.isFetching || undefined}
          className={cn(
            // Batas tinggi membuat header sticky benar-benar menempel saat daftar panjang digulir.
            "scrollbar-thin max-h-[75dvh] overflow-auto transition-opacity",
          isiPenuh && "lg:max-h-none lg:min-h-0",
            server?.isFetching && "opacity-60",
          )}
        >
          {isLoading ? (
            <TableSkeleton cols={Math.min(6, columns.length)} />
          ) : (
            <table className="w-full min-w-[760px] border-separate border-spacing-0 text-[13px]">
              <thead className="sticky top-0 z-10 bg-card">
                {table.getHeaderGroups().map((hg) => (
                  <tr key={hg.id}>
                    {hg.headers.map((h) => {
                      const sortable = h.column.getCanSort();
                      const dir = h.column.getIsSorted();
                      return (
                        <th
                          key={h.id}
                          aria-sort={
                            sortable
                              ? dir === "asc"
                                ? "ascending"
                                : dir === "desc"
                                  ? "descending"
                                  : "none"
                              : undefined
                          }
                          style={{
                            width:
                              h.getSize() !== 150 ? h.getSize() : undefined,
                          }}
                          className={cn(
                            "h-9 border-b border-border bg-secondary px-2.5 text-left text-[11px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase first:pl-4",
                            h.column.columnDef.meta?.sematKanan &&
                              "sticky right-0 z-[2] border-l",
                          )}
                        >
                          {h.isPlaceholder ? null : sortable ? (
                            <button
                              type="button"
                              onClick={
                                server
                                  ? () => server.onSortToggle(h.column.id)
                                  : h.column.getToggleSortingHandler()
                              }
                              className="inline-flex items-center gap-1 tracking-wide uppercase hover:text-foreground"
                            >
                              {flexRender(
                                h.column.columnDef.header,
                                h.getContext(),
                              )}
                              {dir === "asc" ? (
                                <ArrowUp className="size-3" />
                              ) : dir === "desc" ? (
                                <ArrowDown className="size-3" />
                              ) : (
                                <ChevronsUpDown className="size-3 opacity-40" />
                              )}
                            </button>
                          ) : (
                            flexRender(
                              h.column.columnDef.header,
                              h.getContext(),
                            )
                          )}
                        </th>
                      );
                    })}
                  </tr>
                ))}
              </thead>
              <tbody>
                {(kelompok ?? [{ key: "", rows: pageRows }]).map((g) => {
                  const lipat = !!groupBy && terlipat.has(g.key);
                  const tone = groupBy?.tone?.(g.key);
                  return (
                    <GroupRows
                      key={g.key || "_"}
                      header={
                        groupBy ? (
                          <tr className="bg-secondary/40">
                            <td
                              colSpan={table.getVisibleLeafColumns().length}
                              className="h-9 border-b border-border px-4"
                            >
                              <button
                                type="button"
                                aria-expanded={!lipat}
                                onClick={() =>
                                  setTerlipat((cur) => {
                                    const next = new Set(cur);
                                    if (next.has(g.key)) next.delete(g.key);
                                    else next.add(g.key);
                                    return next;
                                  })
                                }
                                className="flex w-full items-center gap-2 text-left text-[13px] font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                              >
                                <ChevronDown
                                  className={cn(
                                    "size-3.5 text-muted-foreground transition-transform duration-200",
                                    lipat && "-rotate-90",
                                  )}
                                  strokeWidth={2}
                                  aria-hidden
                                />
                                {tone && (
                                  <span
                                    aria-hidden
                                    className={cn(
                                      "size-1.5 rounded-full",
                                      toneClass[tone].dot,
                                    )}
                                  />
                                )}
                                {groupBy.label(g.key)}
                                <span className="num rounded bg-card px-1.5 text-[11px] font-medium text-muted-foreground ring-1 ring-border">
                                  {g.rows.length}
                                </span>
                              </button>
                            </td>
                          </tr>
                        ) : null
                      }
                      rows={lipat ? [] : g.rows}
                      render={(row) => {
                        const i = pageRows.indexOf(row);
                        const selected = row.getIsSelected();
                        const toneBaris = rowTone?.(row.original);
                        return (
                          <tr
                            key={row.id}
                            data-state={selected ? "selected" : undefined}
                            onClick={
                              onRowClick
                                ? () => onRowClick(row.original)
                                : undefined
                            }
                            style={{ "--i": i } as React.CSSProperties}
                            className={cn(
                              "group row-in transition-colors",
                              selected
                                ? "bg-data-50 dark:bg-data-500/10"
                                : "hover:bg-secondary/60",
                              onRowClick && "cursor-pointer",
                            )}
                          >
                            {row.getVisibleCells().map((cell, ci) => (
                              <td
                                key={cell.id}
                                className={cn(
                                  "relative h-12 border-b border-border px-2.5 align-middle first:pl-4",
                                  ci === 0 && "w-11",
                                  cell.column.columnDef.meta?.sematKanan &&
                                    "sticky right-0 z-[1] border-l bg-card group-hover:bg-[color-mix(in_srgb,var(--secondary)_60%,var(--card))] group-data-[state=selected]:bg-data-50 dark:group-data-[state=selected]:bg-[color-mix(in_srgb,var(--color-data-500)_10%,var(--card))]",
                                )}
                              >
                                {ci === 0 && (selected || toneBaris) && (
                                  <span
                                    className={cn(
                                      "absolute inset-y-2 left-0 w-0.5 rounded-r-full",
                                      selected
                                        ? "bg-data-600"
                                        : toneBaris && toneClass[toneBaris].bg,
                                    )}
                                  />
                                )}
                                {cell.column.id === "_pilih" ? (
                                  <span
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      onRowCheck(row, i, e.shiftKey);
                                    }}
                                    className="inline-flex"
                                  >
                                    <Checkbox
                                      checked={selected}
                                      aria-label="Pilih baris"
                                    />
                                  </span>
                                ) : (
                                  flexRender(
                                    cell.column.columnDef.cell,
                                    cell.getContext(),
                                  )
                                )}
                              </td>
                            ))}
                          </tr>
                        );
                      }}
                    />
                  );
                })}
              </tbody>
            </table>
          )}
          {!isLoading && pageRows.length === 0 && (
            <EmptyState
              title={empty?.title ?? `Tidak ada ${entityLabel} yang cocok`}
              description={
                empty?.description ??
                "Ubah kata kunci atau filter untuk melihat data lain."
              }
              action={empty?.action}
              className="py-14"
            />
          )}
        </div>
        {/* Indikator ada kolom lain di luar tampilan */}
        {tepi.kiri && (
          <span
            aria-hidden
            className="pointer-events-none absolute inset-y-0 left-0 z-20 w-4 bg-gradient-to-r from-navy-950/10 to-transparent dark:from-black/40"
          />
        )}
        {tepi.kanan && (
          <span
            aria-hidden
            className="pointer-events-none absolute inset-y-0 right-0 z-20 w-4 bg-gradient-to-l from-navy-950/10 to-transparent dark:from-black/40"
          />
        )}
      </div>

      {/* Paginasi */}
      <div className="flex shrink-0 flex-col items-center justify-between gap-2.5 border-t border-border px-4 py-2 text-[13px] text-muted-foreground sm:flex-row sm:gap-3">
        <span className="text-center sm:text-left">
          <span className="num text-foreground">
            {from.toLocaleString("id-ID")}–{to.toLocaleString("id-ID")}
          </span>{" "}
          dari{" "}
          <span className="num text-foreground">
            {totalBaris.toLocaleString("id-ID")}
          </span>{" "}
          {entityLabel}
          {selectedRows.length > 0 && (
            <span className="ml-2 rounded-md bg-data-50 px-2 py-0.5 text-xs text-data-700 dark:bg-data-500/15 dark:text-data-300">
              <span className="num">
                {selectedRows.length.toLocaleString("id-ID")}
              </span>{" "}
              terpilih
            </span>
          )}
        </span>
        <div className="flex items-center gap-2">
          <Select
            value={String(ps)}
            onValueChange={(v: string) =>
              server
                ? server.onPageSizeChange(Number(v))
                : table.setPageSize(Number(v))
            }
          >
            <SelectTrigger
              size="sm"
              className="w-36 rounded-md text-[13px] md:text-[13px]"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {[...new Set([10, 25, 50, 100, ps])]
                .sort((a, b) => a - b)
                .map((n) => (
                <SelectItem key={n} value={String(n)}>
                  {n} / halaman
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() =>
              server
                ? server.onPageChange(server.page - 1)
                : table.previousPage()
            }
            disabled={!table.getCanPreviousPage()}
            aria-label="Halaman sebelumnya"
          >
            <ChevronLeft />
          </Button>
          <span className="num min-w-14 text-center text-foreground">
            {pageIndex + 1} / {jumlahHal}
          </span>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() =>
              server ? server.onPageChange(server.page + 1) : table.nextPage()
            }
            disabled={!table.getCanNextPage()}
            aria-label="Halaman berikutnya"
          >
            <ChevronRight />
          </Button>
        </div>
      </div>

      {bulkActions.length > 0 && (
        <BulkActionBar
          rows={selectedRows}
          actions={bulkActions}
          entityLabel={entityLabel}
          onClear={clear}
        />
      )}
    </div>
  );
}

/** Satu grup baris: header lipat (opsional) lalu baris-barisnya. Fragment agar <tbody> tetap valid. */
function GroupRows<T>({
  header,
  rows,
  render,
}: {
  header: ReactNode;
  rows: Row<T>[];
  render: (row: Row<T>) => ReactNode;
}) {
  return (
    <>
      {header}
      {rows.map((r) => render(r))}
    </>
  );
}
