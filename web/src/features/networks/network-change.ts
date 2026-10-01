import { toast } from "sonner"
import { type NetworkChange, useChangeNetwork } from "./api"

/**
 * Changes a network and reports the progress, as reconfiguring the servers can take a
 * minute when they restart.
 */
export function useNetworkChange(networkId: string) {
  const change = useChangeNetwork(networkId)
  function run(c: NetworkChange, messages: { loading: string; success: string }, onSuccess?: () => void) {
    toast.promise(change.mutateAsync(c).then(onSuccess), { ...messages, error: (e: Error) => e.message })
  }
  return { run, isPending: change.isPending }
}
