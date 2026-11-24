"use client";

import * as React from "react";
import { CheckIcon, ChevronDownIcon, ChevronUpIcon, SearchIcon, XIcon } from "lucide-react";
import { Popover as PopoverPrimitive } from "radix-ui";
import { cn } from "@/lib/utils";

interface OptionMeta {
  value: string;
  label: React.ReactNode;
  text: string;
}

interface SelectContextValue {
  value: string | undefined;
  onSelect: (value: string) => void;
  open: boolean;
  setOpen: (open: boolean) => void;
  disabled?: boolean;
  listboxId: string;
  search: string;
  setSearch: (q: string) => void;
  labels: Map<string, React.ReactNode>;
  texts: Map<string, string>;
  contentNodeRef: React.MutableRefObject<HTMLDivElement | null>;
  activeValue: string | null;
  setActiveValue: (v: string | null) => void;
}

const SelectContext = React.createContext<SelectContextValue | null>(null);

function useSelectContext(komponen: string): SelectContextValue {
  const ctx = React.useContext(SelectContext);
  if (!ctx) throw new Error(`${komponen} harus digunakan di dalam <Select>`);
  return ctx;
}

function textFromNode(node: React.ReactNode): string {
  if (node == null || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textFromNode).join(" ").trim();
  if (React.isValidElement<{ children?: React.ReactNode }>(node)) {
    return textFromNode(node.props.children);
  }
  return "";
}

function kumpulkanOpsi(
  children: React.ReactNode,
  labels: Map<string, React.ReactNode>,
  texts: Map<string, string>,
): void {
  React.Children.forEach(children, (child) => {
    if (!React.isValidElement<Record<string, unknown>>(child)) return;
    if (child.type === SelectItem && typeof child.props.value === "string") {
      const val = child.props.value;
      const content = child.props.children as React.ReactNode;
      const explicit = typeof child.props.textValue === "string" ? child.props.textValue : "";
      labels.set(val, content);
      texts.set(val, `${explicit} ${textFromNode(content)} ${val}`.trim().toLowerCase());
      return;
    }
    if (child.props?.children) {
      kumpulkanOpsi(child.props.children as React.ReactNode, labels, texts);
    }
  });
}

function cocokPencarian(
  value: string,
  children: React.ReactNode,
  textValue: string | undefined,
  texts: Map<string, string>,
  query: string,
): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  const cached = texts.get(value);
  const target = cached ?? `${textValue ?? ""} ${textFromNode(children)} ${value}`.toLowerCase();
  return target.includes(q);
}

function hitungOpsiCocok(
  children: React.ReactNode,
  texts: Map<string, string>,
  query: string,
): number {
  const q = query.trim().toLowerCase();
  let total = 0;
  React.Children.forEach(children, (child) => {
    if (!React.isValidElement<Record<string, unknown>>(child)) return;
    if (child.type === SelectItem && typeof child.props.value === "string") {
      if (
        !q ||
        cocokPencarian(
          child.props.value,
          child.props.children as React.ReactNode,
          typeof child.props.textValue === "string" ? child.props.textValue : undefined,
          texts,
          q,
        )
      ) {
        total += 1;
      }
      return;
    }
    if (child.props?.children) {
      total += hitungOpsiCocok(child.props.children as React.ReactNode, texts, q);
    }
  });
  return total;
}

interface SelectProps {
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  disabled?: boolean;
  name?: string;
  required?: boolean;
  children?: React.ReactNode;
}

/**
 * Combo box / Select bergaya Clean Clinical yang lembut dengan pencarian bawaan.
 * Mendukung API komponen `<Select>` standar (`SelectTrigger`, `SelectValue`, `SelectContent`, `SelectItem`).
 */
function Select({
  value: controlledValue,
  defaultValue,
  onValueChange,
  open: controlledOpen,
  defaultOpen = false,
  onOpenChange,
  disabled,
  name,
  required,
  children,
}: SelectProps) {
  const [uncontrolledValue, setUncontrolledValue] = React.useState<string | undefined>(defaultValue);
  const [uncontrolledOpen, setUncontrolledOpen] = React.useState(defaultOpen);
  const [search, setSearch] = React.useState("");
  const [activeValue, setActiveValue] = React.useState<string | null>(null);
  const contentNodeRef = React.useRef<HTMLDivElement | null>(null);
  const listboxId = React.useId();

  const value = controlledValue !== undefined ? controlledValue : uncontrolledValue;
  const open = controlledOpen !== undefined ? controlledOpen : uncontrolledOpen;

  const setOpen = React.useCallback(
    (next: boolean) => {
      if (disabled && next) return;
      if (controlledOpen === undefined) setUncontrolledOpen(next);
      onOpenChange?.(next);
      if (!next) {
        setSearch("");
        setActiveValue(null);
      }
    },
    [controlledOpen, disabled, onOpenChange],
  );

  const onSelect = React.useCallback(
    (nextVal: string) => {
      if (controlledValue === undefined) setUncontrolledValue(nextVal);
      onValueChange?.(nextVal);
      setOpen(false);
    },
    [controlledValue, onValueChange, setOpen],
  );

  const { labels, texts } = React.useMemo(() => {
    const l = new Map<string, React.ReactNode>();
    const t = new Map<string, string>();
    kumpulkanOpsi(children, l, t);
    return { labels: l, texts: t };
  }, [children]);

  const ctx = React.useMemo<SelectContextValue>(
    () => ({
      value,
      onSelect,
      open,
      setOpen,
      disabled,
      listboxId,
      search,
      setSearch,
      labels,
      texts,
      contentNodeRef,
      activeValue,
      setActiveValue,
    }),
    [value, onSelect, open, setOpen, disabled, listboxId, search, labels, texts, activeValue],
  );

  return (
    <SelectContext.Provider value={ctx}>
      <PopoverPrimitive.Root open={open} onOpenChange={setOpen}>
        {name && <input type="hidden" name={name} value={value ?? ""} required={required} />}
        {children}
      </PopoverPrimitive.Root>
    </SelectContext.Provider>
  );
}

function SelectGroup({
  children,
  className,
  ...props
}: React.ComponentProps<"div">) {
  const { texts, search } = useSelectContext("SelectGroup");
  if (search.trim() && hitungOpsiCocok(children, texts, search) === 0) {
    return null;
  }
  return (
    <div data-slot="select-group" role="group" className={cn("space-y-0.5", className)} {...props}>
      {children}
    </div>
  );
}

function SelectValue({
  placeholder,
  className,
  children,
}: {
  placeholder?: React.ReactNode;
  className?: string;
  children?: React.ReactNode;
}) {
  const { value, labels } = useSelectContext("SelectValue");
  const label = value !== undefined ? labels.get(value) : undefined;
  const tampil = children ?? (label !== undefined ? label : placeholder);
  const kosong = label === undefined && children === undefined;

  return (
    <span
      data-slot="select-value"
      data-placeholder={kosong ? "" : undefined}
      className={cn(kosong && "text-muted-foreground", className)}
    >
      {tampil}
    </span>
  );
}

function SelectTrigger({
  className,
  size = "default",
  children,
  disabled: propDisabled,
  type = "button",
  ...props
}: React.ComponentProps<"button"> & {
  size?: "sm" | "default";
}) {
  const { open, disabled: ctxDisabled, listboxId } = useSelectContext("SelectTrigger");
  const isDisabled = propDisabled ?? ctxDisabled;

  return (
    <PopoverPrimitive.Trigger asChild>
      <button
        type={type}
        role="combobox"
        aria-expanded={open}
        aria-controls={listboxId}
        aria-haspopup="listbox"
        disabled={isDisabled}
        data-slot="select-trigger"
        data-size={size}
        data-state={open ? "open" : "closed"}
        className={cn(
          "group/select flex w-full max-w-full min-w-0 items-center justify-between gap-2 rounded-xl border border-border/80 bg-card/95 px-3 py-2 text-base whitespace-nowrap shadow-[0_1px_2px_rgba(15,23,42,0.03)] outline-none transition-all duration-150 select-none hover:not-disabled:border-border hover:not-disabled:bg-secondary/45 focus-visible:border-ring/60 focus-visible:ring-[3px] focus-visible:ring-ring/20 data-[state=open]:border-ring/60 data-[state=open]:bg-card data-[state=open]:ring-[3px] data-[state=open]:ring-ring/15 disabled:cursor-not-allowed disabled:bg-muted/70 disabled:opacity-60 aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/15 data-[size=default]:h-9 data-[size=sm]:h-8 md:text-sm *:data-[slot=select-value]:block *:data-[slot=select-value]:min-w-0 *:data-[slot=select-value]:flex-1 *:data-[slot=select-value]:truncate *:data-[slot=select-value]:text-left dark:border-white/10 dark:bg-card/85 dark:hover:not-disabled:border-white/20 dark:hover:not-disabled:bg-white/[0.06] dark:aria-invalid:ring-destructive/30 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4 [&_svg:not([class*='text-'])]:text-muted-foreground",
          className,
        )}
        {...props}
      >
        {children}
        <ChevronDownIcon className="size-3.5 opacity-55 transition-transform duration-150 group-data-[state=open]/select:rotate-180" />
      </button>
    </PopoverPrimitive.Trigger>
  );
}

function SelectContent({
  className,
  children,
  align = "start",
  sideOffset = 6,
  collisionPadding = 8,
  searchable = true,
  searchPlaceholder = "Cari pilihan…",
  emptyText = "Tidak ada pilihan yang cocok.",
  ...props
}: Omit<React.ComponentProps<typeof PopoverPrimitive.Content>, "position"> & {
  position?: "popper" | "item-aligned";
  searchable?: boolean;
  searchPlaceholder?: string;
  emptyText?: string;
}) {
  const { listboxId, search, setSearch, texts, contentNodeRef, activeValue, setActiveValue } =
    useSelectContext("SelectContent");
  const searchInputRef = React.useRef<HTMLInputElement | null>(null);

  const visibleCount = React.useMemo(
    () => (search.trim() ? hitungOpsiCocok(children, texts, search) : null),
    [children, texts, search],
  );

  // Mencegah FocusScope Dialog mencuri fokus dari kolom cari di dalam Popover portal.
  const setNodeRef = React.useCallback(
    (node: HTMLDivElement | null) => {
      contentNodeRef.current = node;
    },
    [contentNodeRef],
  );

  React.useEffect(() => {
    const el = contentNodeRef.current;
    if (!el) return;
    const tahanFokus = (e: FocusEvent) => {
      e.stopPropagation();
    };
    el.addEventListener("focusin", tahanFokus);
    return () => el.removeEventListener("focusin", tahanFokus);
  }, [contentNodeRef]);

  function dapatkanOpsiAktif(): HTMLElement[] {
    const el = contentNodeRef.current;
    if (!el) return [];
    return [...el.querySelectorAll<HTMLElement>('[data-slot="select-item"]:not([aria-disabled="true"])')];
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      const opsi = dapatkanOpsiAktif();
      if (opsi.length === 0) return;
      e.preventDefault();
      const idx = activeValue
        ? opsi.findIndex((o) => o.dataset.value === activeValue)
        : -1;
      const nextIdx =
        e.key === "ArrowDown"
          ? (idx + 1) % opsi.length
          : (idx - 1 + opsi.length) % opsi.length;
      const target = opsi[nextIdx];
      setActiveValue(target.dataset.value ?? null);
      target.scrollIntoView({ block: "nearest" });
      return;
    }
    if (e.key === "Enter" && document.activeElement === searchInputRef.current) {
      const opsi = dapatkanOpsiAktif();
      if (opsi.length === 0) return;
      e.preventDefault();
      const target = (activeValue && opsi.find((o) => o.dataset.value === activeValue)) || opsi[0];
      target.click();
    }
  }

  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Content
        ref={setNodeRef}
        data-slot="select-content"
        data-lenis-prevent
        align={align}
        sideOffset={sideOffset}
        collisionPadding={collisionPadding}
        onOpenAutoFocus={(e) => {
          if (!searchable) return;
          e.preventDefault();
          searchInputRef.current?.focus({ preventScroll: true });
        }}
        onKeyDown={handleKeyDown}
        className={cn(
          "pointer-events-auto relative z-[70] flex w-auto min-w-[max(var(--radix-popover-trigger-width),12rem)] max-w-[calc(100vw-1rem)] origin-(--radix-popover-content-transform-origin) flex-col overflow-hidden rounded-xl border border-border/75 bg-popover/98 p-0 text-popover-foreground shadow-[0_12px_32px_-8px_rgba(15,23,42,0.12),0_4px_12px_-4px_rgba(15,23,42,0.06)] backdrop-blur-md outline-none data-[side=bottom]:slide-in-from-top-1.5 data-[side=left]:slide-in-from-right-1.5 data-[side=right]:slide-in-from-left-1.5 data-[side=top]:slide-in-from-bottom-1.5 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-[0.98] data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-[0.98] dark:border-white/12 dark:shadow-[0_16px_40px_-10px_rgba(0,0,0,0.55)]",
          className,
        )}
        {...props}
      >
        {searchable && (
          <div
            data-slot="select-search"
            className="border-b border-border/60 bg-muted/30 px-2 py-2 dark:bg-white/[0.02]"
          >
            <div className="flex h-8 items-center gap-1.5 rounded-lg border border-border/70 bg-background/90 px-2.5 transition-colors focus-within:border-ring/55 focus-within:bg-background focus-within:ring-2 focus-within:ring-ring/15 dark:border-white/10 dark:bg-background/60">
              <SearchIcon className="size-3.5 shrink-0 text-muted-foreground/75" aria-hidden />
              <input
                ref={searchInputRef}
                type="text"
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setActiveValue(null);
                }}
                placeholder={searchPlaceholder}
                aria-label={searchPlaceholder}
                className="min-w-0 flex-1 bg-transparent text-[13px] text-foreground outline-none placeholder:text-muted-foreground/75"
              />
              {search && (
                <button
                  type="button"
                  onClick={() => {
                    setSearch("");
                    setActiveValue(null);
                    searchInputRef.current?.focus({ preventScroll: true });
                  }}
                  aria-label="Bersihkan pencarian"
                  className="grid size-5 shrink-0 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
                >
                  <XIcon className="size-3" />
                </button>
              )}
            </div>
          </div>
        )}
        <div
          id={listboxId}
          role="listbox"
          className="scrollbar-thin max-h-60 overflow-y-auto p-1.5 space-y-0.5"
        >
          {children}
          {visibleCount === 0 && (
            <p className="px-3 py-4 text-center text-xs text-muted-foreground">
              {emptyText}
            </p>
          )}
        </div>
      </PopoverPrimitive.Content>
    </PopoverPrimitive.Portal>
  );
}

function SelectLabel({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="select-label"
      className={cn("px-2.5 py-1.5 text-[11px] font-medium tracking-wide text-muted-foreground", className)}
      {...props}
    />
  );
}

function SelectItem({
  className,
  children,
  value,
  textValue,
  disabled,
  ...props
}: Omit<React.ComponentProps<"button">, "value" | "onSelect"> & {
  value: string;
  textValue?: string;
}) {
  const { value: selectedValue, onSelect, search, texts, activeValue, setActiveValue } =
    useSelectContext("SelectItem");

  if (!cocokPencarian(value, children, textValue, texts, search)) {
    return null;
  }

  const isSelected = selectedValue === value;
  const isActive = activeValue === value;

  return (
    <button
      type="button"
      role="option"
      aria-selected={isSelected}
      aria-disabled={disabled || undefined}
      disabled={disabled}
      data-slot="select-item"
      data-value={value}
      data-selected={isSelected ? "true" : undefined}
      data-active={isActive ? "true" : undefined}
      onMouseEnter={() => !disabled && setActiveValue(value)}
      onClick={() => {
        if (!disabled) onSelect(value);
      }}
      className={cn(
        "relative flex w-full cursor-pointer items-center gap-2 rounded-lg py-1.5 pr-8 pl-2.5 text-left text-[13px] outline-none select-none transition-colors duration-100 hover:bg-secondary/75 focus-visible:bg-secondary/75 data-[active=true]:bg-secondary/75 data-[selected=true]:bg-accent/70 data-[selected=true]:font-medium data-[selected=true]:text-accent-foreground disabled:pointer-events-none disabled:opacity-45 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4 [&_svg:not([class*='text-'])]:text-muted-foreground",
        className,
      )}
      {...props}
    >
      <span
        data-slot="select-item-indicator"
        className="absolute right-2.5 flex size-3.5 items-center justify-center"
      >
        {isSelected && <CheckIcon className="size-3.5 text-accent-foreground" />}
      </span>
      <span className="min-w-0 flex-1 truncate">{children}</span>
    </button>
  );
}

function SelectSeparator({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="select-separator"
      role="separator"
      className={cn("pointer-events-none -mx-1 my-1 h-px bg-border/60", className)}
      {...props}
    />
  );
}

function SelectScrollUpButton({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="select-scroll-up-button"
      className={cn("hidden items-center justify-center py-1", className)}
      {...props}
    >
      <ChevronUpIcon className="size-4" />
    </div>
  );
}

function SelectScrollDownButton({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="select-scroll-down-button"
      className={cn("hidden items-center justify-center py-1", className)}
      {...props}
    >
      <ChevronDownIcon className="size-4" />
    </div>
  );
}

export {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectScrollDownButton,
  SelectScrollUpButton,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
  type OptionMeta,
};
