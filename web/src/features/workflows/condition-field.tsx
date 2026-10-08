import { BracketsSquareIcon, PlusIcon, XIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Segmented } from "@/components/segmented"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { cn } from "@/lib/utils"
import type { Condition, Op, Rule } from "./api"
import { binary, ops } from "./catalog"
import { TemplateInput } from "./template-input"

const maxRules = 20
const maxDepth = 3

/** Edits a condition: rules that compare two values, all or any of which must hold, and groups of rules. */
export function ConditionField({ value, onChange, depth = 1 }: { value: Condition; onChange: (c: Condition) => void; depth?: number }) {
  const set = (i: number, rule: Rule) => onChange({ ...value, rules: value.rules.with(i, rule) })
  const add = (rule: Rule) => onChange({ ...value, rules: [...value.rules, rule] })
  return (
    <div className={cn("grid gap-3", depth > 1 && "rounded-lg bg-muted/40 p-3 ring-1 ring-border")}>
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="text-muted-foreground">{t("Holds if")}</span>
        <Segmented
          label={t("Holds if")}
          value={value.match}
          onChange={(match) => onChange({ ...value, match })}
          options={[
            { value: "all", label: t("all rules hold") },
            { value: "any", label: t("any rule holds") },
          ]}
        />
      </div>
      <ol className="grid gap-2">
        {value.rules.map((rule, i) => (
          // Rules have no identity of their own; their fields are controlled.
          <li key={i} className="flex items-start gap-2">
            <div className="min-w-0 flex-1">
              {rule.group ? (
                <ConditionField value={rule.group} onChange={(group) => set(i, { group })} depth={depth + 1} />
              ) : (
                <RuleField rule={rule} onChange={(r) => set(i, r)} />
              )}
            </div>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label={t("Remove rule {{number}}", { number: i + 1 })}
              disabled={value.rules.length === 1}
              onClick={() => onChange({ ...value, rules: value.rules.toSpliced(i, 1) })}
            >
              <XIcon />
            </Button>
          </li>
        ))}
      </ol>
      <div className="flex flex-wrap gap-2">
        <Button type="button" size="sm" variant="outline" disabled={value.rules.length >= maxRules} onClick={() => add({ left: "", op: "eq", right: "" })}>
          <PlusIcon />
          {t("Rule")}
        </Button>
        {depth < maxDepth && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={value.rules.length >= maxRules}
            onClick={() => add({ group: { match: value.match === "all" ? "any" : "all", rules: [{ left: "", op: "eq", right: "" }] } })}
          >
            <BracketsSquareIcon />
            {t("Group")}
          </Button>
        )}
      </div>
    </div>
  )
}

function RuleField({ rule, onChange }: { rule: Rule; onChange: (rule: Rule) => void }) {
  return (
    <div className="grid gap-2 rounded-lg sm:grid-cols-[11rem_1fr]">
      <div className="sm:col-span-2">
        <TemplateInput aria-label={t("Value")} placeholder={t("e.g. {{example}}", { example: "{{steps.find.count}}" })} value={rule.left ?? ""} onChange={(left) => onChange({ ...rule, left })} />
      </div>
      <Select value={rule.op ?? "eq"} onValueChange={(op) => onChange({ ...rule, op: op as Op })}>
        <SelectTrigger aria-label={t("Comparison")} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {Object.entries(ops).map(([op, label]) => (
            <SelectItem key={op} value={op}>
              {t(label)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {binary(rule.op) ? (
        <TemplateInput aria-label={t("Compared with")} value={rule.right ?? ""} onChange={(right) => onChange({ ...rule, right })} />
      ) : null}
    </div>
  )
}
