import { getRouteApi } from "@tanstack/react-router"
import { useMemo } from "react"
import { useServer } from "@/features/servers/api"
import { FileBrowser } from "./file-browser"
import { FileEditor } from "./file-editor"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/files")

/** The Files tab of a server: a folder, or a file in the editor. */
export function FilesPage() {
  const { nodeId, serverId } = route.useParams()
  const { path = "", edit } = route.useSearch()
  const navigate = route.useNavigate()
  const { server } = useServer(nodeId, serverId)
  const files = useMemo(() => ({ nodeId, serverId }), [nodeId, serverId])

  if (!server) return null
  return edit ? (
    <FileEditor key={edit} files={files} path={edit} onClose={() => navigate({ search: { path: path || undefined } })} />
  ) : (
    <FileBrowser files={files} path={path} server={server} />
  )
}
