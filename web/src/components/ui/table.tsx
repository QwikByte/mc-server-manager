"use client"

import * as React from "react"
import { CaretDownIcon, CaretUpDownIcon, CaretUpIcon } from "@phosphor-icons/react"
import { cn } from "cn"
import type { Sorting } from "@/lib/sort"

function Table({ className, ...props }: React.ComponentProps<"table">) {
  return (
    <div
      data-slot="table-container"
      className="relative w-full overflow-x-auto"
    >
      <table
        data-slot="table"
        className={cn("w-full caption-bottom text-sm", className)}
        {...props}
      />
    </div>
  )
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("[&_tr]:border-b", className)}
      {...props}
    />
  )
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  )
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        "border-t bg-muted/50 font-medium [&>tr]:last:border-b-0",
        className
      )}
      {...props}
    />
  )
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "border-b transition-colors hover:bg-muted/50 has-aria-expanded:bg-muted/50 data-[state=selected]:bg-muted",
        className
      )}
      {...props}
    />
  )
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "h-10 px-3 text-left align-middle text-xs font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase [&:has([role=checkbox])]:pr-0",
        className
      )}
      {...props}
    />
  )
}

/**
 * The header of a column that sorts the table: a click sorts by the column, or the other way
 * around if the table is sorted by it already.
 */
function SortableHead<K extends string>({
  sorting,
  column,
  children,
  ...props
}: React.ComponentProps<"th"> & { sorting: Sorting<K>; column: K }) {
  const order = sorting.by === column ? sorting.order : undefined
  const Icon = order === "asc" ? CaretUpIcon : order === "desc" ? CaretDownIcon : CaretUpDownIcon
  return (
    <TableHead aria-sort={order && (order === "asc" ? "ascending" : "descending")} {...props}>
      <button
        type="button"
        onClick={() => sorting.sort(column, order && (order === "asc" ? "desc" : "asc"))}
        className={cn(
          "-mx-1.5 inline-flex h-8 items-center gap-1 rounded-md px-1.5 [text-transform:inherit] outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring",
          order && "text-foreground"
        )}
      >
        {children}
        <Icon aria-hidden weight="bold" className={cn("size-3 shrink-0", !order && "opacity-50")} />
      </button>
    </TableHead>
  )
}

function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn(
        "px-3 py-2.5 align-middle whitespace-nowrap [&:has([role=checkbox])]:pr-0",
        className
      )}
      {...props}
    />
  )
}

function TableCaption({
  className,
  ...props
}: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  SortableHead,
  TableRow,
  TableCell,
  TableCaption,
}
