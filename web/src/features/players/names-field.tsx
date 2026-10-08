import { XIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { maxPlayers } from "./api"
import { add, type Names, namesOf } from "./names"
import { PlayerAvatar } from "./player-name"

/** Asks for up to maxPlayers names of players, which Enter, a space or a comma add, and suggests those the servers know. */
export function NamesField({ id, value, onChange, known }: { id: string; value: Names; onChange: (value: Names) => void; known: string[] }) {
  const { names, input } = value
  const typed = input.trim().toLowerCase()
  const suggestions = typed
    ? known.filter((n) => n.toLowerCase().includes(typed) && !names.some((m) => m.toLowerCase() === n.toLowerCase())).slice(0, 6)
    : []
  return (
    <div className="grid gap-2">
      {names.length > 0 && (
        <ul className="flex flex-wrap gap-1.5" aria-label={t("Players")}>
          {names.map((name) => (
            <li key={name} className="inline-flex items-center gap-1.5 rounded-md bg-muted py-0.5 pr-0.5 pl-1 text-xs font-medium">
              <PlayerAvatar name={name} className="size-5 rounded text-[0.6rem]" />
              <span className="font-mono">{name}</span>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                aria-label={t("Remove {{name}}", { name })}
                onClick={() => onChange({ names: names.filter((n) => n !== name), input })}
              >
                <XIcon />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <Input
        id={id}
        autoFocus
        autoComplete="off"
        className="font-mono"
        placeholder={names.length > 0 ? t("More players") : t("e.g. Steve, .Alex")}
        disabled={names.length >= maxPlayers}
        value={input}
        aria-invalid={input.trim() !== "" && namesOf(value).length === names.length}
        onChange={(e) => {
          const text = e.target.value
          onChange(/[\s,;]/.test(text) ? add(names, text) : { names, input: text })
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter" && input.trim()) {
            e.preventDefault()
            onChange(add(names, input))
          } else if (e.key === "Backspace" && !input && names.length > 0) {
            onChange({ names: names.slice(0, -1), input })
          }
        }}
      />
      {suggestions.length > 0 && (
        <div className="flex flex-wrap gap-1.5" aria-label={t("Suggestions")}>
          {suggestions.map((name) => (
            <Button key={name} type="button" variant="outline" size="xs" className="font-mono" onClick={() => onChange(add(names, name))}>
              {name}
            </Button>
          ))}
        </div>
      )}
    </div>
  )
}
