import { ImageIcon, TrashIcon, UploadSimpleIcon } from "@phosphor-icons/react"
import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { useRef } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { useAccess } from "@/features/access/use-access"
import { contentUrl, type ServerFiles, upload, useChangeFiles } from "@/features/files/api"
import { responseError } from "@/lib/api"

const file = "server-icon.png"
/** Minecraft shows icons of 64×64 pixels in the server list. */
const size = 64
/** Bigger files aren't shown; an icon has a few kilobytes. */
const maxShownBytes = 256 * 1024

const dataUrl = (blob: Blob) =>
  new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(blob)
  })

/**
 * The icon of a server as a data: URL, which the panel's Content Security Policy allows for
 * images, unlike the attachments of the file manager; null if there is none.
 */
const iconQuery = (s: ServerFiles) =>
  queryOptions({
    queryKey: ["files", s.nodeId, s.serverId, file, "icon"],
    queryFn: async () => {
      const res = await fetch(contentUrl(s, file))
      if (res.status === 404) return null
      if (!res.ok) throw await responseError(res)
      if (Number(res.headers.get("Content-Length")) > maxShownBytes) {
        await res.body?.cancel()
        return null
      }
      return dataUrl(new Blob([await res.arrayBuffer()], { type: "image/png" }))
    },
  })

/** Scales an image to the PNG Minecraft wants, cut to a square in its middle. Small images stay sharp, e.g. pixel art. */
async function toIcon(image: File) {
  const bitmap = await createImageBitmap(image).catch(() => {
    throw new Error(t("{{name}} isn't an image the browser can read.", { name: image.name }))
  })
  const side = Math.min(bitmap.width, bitmap.height)
  const canvas = new OffscreenCanvas(size, size)
  const context = canvas.getContext("2d")!
  context.imageSmoothingEnabled = side > size
  context.imageSmoothingQuality = "high"
  context.drawImage(bitmap, (bitmap.width - side) / 2, (bitmap.height - side) / 2, side, side, 0, 0, size, size)
  bitmap.close()
  return canvas.convertToBlob({ type: "image/png" })
}

/**
 * The icon of a server in the server list of the game, as server-icon.png in its folder. The
 * browser scales an uploaded image to 64×64 pixels. Needs the permission to change files.
 */
export function ServerIcon({ nodeId, serverId }: ServerFiles) {
  const { can } = useAccess()
  const s = { nodeId, serverId }
  const queryClient = useQueryClient()
  const { data: icon } = useQuery({ ...iconQuery(s), enabled: can("files.read", nodeId, serverId) })
  const save = useMutation({
    mutationFn: async (image: File) => upload(s, file, await toIcon(image), { overwrite: true }),
    onSuccess: () => toast.success(t("Saved the icon. The server shows it after its next start.")),
    onError: (e) => toast.error(e.message),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["files", nodeId, serverId] }),
  })
  const change = useChangeFiles(s)
  const picker = useRef<HTMLInputElement>(null)
  const busy = save.isPending || change.isPending

  return (
    <div className="flex items-center gap-4 md:col-span-2">
      <div className="grid size-16 shrink-0 place-items-center overflow-hidden rounded-lg bg-muted ring-1 ring-foreground/10">
        {icon ? (
          <img src={icon} alt={t("Server icon")} width={size} height={size} className="[image-rendering:pixelated]" />
        ) : (
          <ImageIcon className="size-6 text-muted-foreground" weight="duotone" aria-hidden />
        )}
      </div>
      <div className="min-w-0 space-y-2">
        <div>
          <p className="text-sm font-medium">{t("Server icon")}</p>
          <p className="text-sm text-muted-foreground">{t("Shown next to the MOTD in the server list. Images are scaled to 64×64 pixels.")}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => picker.current?.click()}>
            <UploadSimpleIcon />
            {icon ? t("Replace icon…") : t("Upload icon…")}
          </Button>
          {icon && (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              disabled={busy}
              className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
              onClick={() => change.mutate({ action: "delete", path: file }, { onError: (e) => toast.error(e.message) })}
            >
              <TrashIcon />
              {t("Remove")}
            </Button>
          )}
        </div>
        <input
          ref={picker}
          type="file"
          accept="image/png,image/jpeg,image/webp,image/gif"
          hidden
          onChange={(e) => {
            const image = e.target.files?.[0]
            if (image) save.mutate(image)
            e.target.value = ""
          }}
        />
      </div>
    </div>
  )
}
