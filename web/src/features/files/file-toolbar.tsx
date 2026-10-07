import {
  CaretDownIcon,
  FilePlusIcon,
  FolderPlusIcon,
  FolderSimpleIcon,
  FilesIcon,
  MagnifyingGlassIcon,
  PlusIcon,
  SortAscendingIcon,
  UploadSimpleIcon,
} from "@phosphor-icons/react"
import { t } from "i18next"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { type Sort, sorts } from "./browse"

/** Creates a file or folder, or uploads files or a whole folder. */
export function FileMenus({ onCreate, onUpload }: { onCreate: (kind: "file" | "folder") => void; onUpload: (folder: boolean) => void }) {
  return (
    <div className="flex gap-2">
      {/* Not modal, so that focus moves to the dialogs opened from it. */}
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button variant="outline">
            <PlusIcon />
            {t("New")}
            <CaretDownIcon className="opacity-60" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onSelect={() => onCreate("file")}>
            <FilePlusIcon />
            {t("New file")}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => onCreate("folder")}>
            <FolderPlusIcon />
            {t("New folder")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button>
            <UploadSimpleIcon />
            {t("Upload")}
            <CaretDownIcon className="opacity-60" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onSelect={() => onUpload(false)}>
            <FilesIcon />
            {t("Files…")}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => onUpload(true)}>
            <FolderSimpleIcon />
            {t("Folder…")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}

/** Filters a folder by name and sorts it. */
export function FilterBar({
  filter,
  onFilter,
  sort,
  onSort,
}: {
  filter: string
  onFilter: (filter: string) => void
  sort: Sort
  onSort: (sort: Sort) => void
}) {
  return (
    <div className="flex items-center gap-2 border-b px-4 py-2.5">
      <InputGroup className="sm:max-w-xs">
        <InputGroupAddon>
          <MagnifyingGlassIcon />
        </InputGroupAddon>
        <InputGroupInput
          type="search"
          placeholder={t("Filter by name")}
          aria-label={t("Filter by name")}
          value={filter}
          onChange={(e) => onFilter(e.target.value)}
        />
      </InputGroup>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" className="ml-auto" aria-label={`${t("Sort")}: ${t(sorts[sort])}`}>
            <SortAscendingIcon />
            <span className="max-sm:hidden">{t(sorts[sort])}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuLabel>{t("Sort")}</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={sort} onValueChange={(value) => onSort(value as Sort)}>
            {(Object.keys(sorts) as Sort[]).map((value) => (
              <DropdownMenuRadioItem key={value} value={value}>
                {t(sorts[value])}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
