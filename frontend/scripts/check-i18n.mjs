// check-i18n: finds hardcoded user-facing strings in TSX that should use the i18n dictionary.
// Run: npm run check:i18n — exits 1 with file:line findings when untranslated UI text is found.
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const SRC = join(fileURLToPath(new URL('..', import.meta.url)), 'src')

// Technical or brand strings that are intentionally not translated.
// Keep this list short: every entry should have a reason.
const ALLOW = new Set([
  'AI', 'API', 'CSV', 'HTML', 'JSON', 'SUS', 'TL;DR', 'URL', 'XML',
  'DS:', 'AI:', '×', '↓', '↑', '⇅', '…', '—', '•',
  'All', // select option value in filters, kept per convention
  // brand names shown as-is in both languages (issue tracker providers)
  'GitHub', 'GitLab', 'Bugzilla', 'Azure Boards', 'Jira',
  // keyboard shortcut labels (kbd spans) and key names
  'P', 'F', 'B', 'S', 'ESC', 'Ctrl+Enter',
  // TS type leaking into the text-node regex (fn: () => Promise<void>)
  'Promise',
])

// placeholder values that are technical samples, not UI copy
const isTechnicalPlaceholder = (v) =>
  /^https?:\/\//.test(v) || v.includes('@example.') || v === 'https://...'

const findings = []

function walk(dir, out) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) {
      walk(full, out)
      continue
    }
    if (full.endsWith('.tsx') && !full.endsWith('.test.tsx')) out.push(full)
  }
}

const files = []
walk(join(SRC, 'features'), files)
walk(join(SRC, 'components'), files)

const ATTR = /\b(placeholder|aria-label|title)="([^"{}]*)"/
const CAPS = /[A-ZŁŚŻŹĆÓŃĄ]/

for (const file of files) {
  const rel = file.slice(SRC.length + 1).replaceAll('\\', '/')
  const lines = readFileSync(file, 'utf8').split(/\r?\n/)
  lines.forEach((line, i) => {
    // literal string attribute values with letters
    let m = ATTR.exec(line)
    while (m) {
      const val = m[2].trim()
      if (val && /[A-Za-zĄ-Żą-ż]/.test(val) && !ALLOW.has(val) && !isTechnicalPlaceholder(val)) {
        findings.push(`${rel}:${i + 1} attribute ${m[1]}="${val}"`)
      }
      m = ATTR.exec(line.slice(m.index + m[0].length))
    }
    // notify(...) with a literal first argument
    m = /notify(Action)?\(\s*'([^']+)'/.exec(line) || /notify(Action)?\(\s*`([^`$]*)`/.exec(line)
    if (m) {
      const val = (m[2] || '').trim()
      if (val && !ALLOW.has(val)) findings.push(`${rel}:${i + 1} notify('${val}'`)
    }
    // visible JSX text nodes that start with a capital letter
    m = />([^<>{}]*[A-ZŁŚŻŹĆÓŃĄ][^<>{}]*)</.exec(line)
    if (m) {
      const val = m[1].trim()
      if (val && CAPS.test(val[0]) && !ALLOW.has(val) && !ALLOW.has(val.replace(/\s*\(.*\)$/, ''))) {
        findings.push(`${rel}:${i + 1} text "${val}"`)
      }
    }
  })
}

if (findings.length > 0) {
  console.error(`check-i18n: ${findings.length} hardcoded UI string(s) found (move them to src/i18n/index.tsx or extend the allowlist with a reason):`)
  for (const f of findings) console.error('  ' + f)
  process.exit(1)
}
console.log('check-i18n: no hardcoded UI strings found')
