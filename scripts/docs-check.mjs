#!/usr/bin/env node
/**
 * docs-check.mjs —— 文档一致性与合规自检（对齐 AI-STD-006 仓库 Harness）
 *
 * 用法：pnpm run docs:check（等价的 make 目标：make docs-check）
 *
 * 为什么存在：本轮产品研究暴露出「文档说了、图没跟上」「需求有编号、任务没承接」
 * 「mermaid 改了、PNG 没重渲染」这类漂移——它们不会被编译器发现，只会误导下一个智能体。
 * 把它们变成一条可复跑的检查，比每次手工 grep 可靠。
 *
 * 检查项（E = 错误，退出码 1；W = 告警，不改变退出码）：
 *   E1 相对链接可解析（docs/**\/*.md + 根 README.md / AGENTS.md）
 *   E2 FR 编号双向一致：AI-PRD 的 FR 必须被 exec-plans 承接，反之亦然
 *   E3 docs/ui 的 mermaid 源码与渲染 PNG 成对，且 PNG 不早于源码（防图源脱节）
 *   E4 exec-plans 的验收用例 JSON 块可解析且字段完整
 *   W1 每份文档开头标注 AI-STD 条款（仓库文档约定）
 *   W2 prettier 格式（按仓库现行的 CRLF 行尾校验，见 --end-of-line 说明）
 */
import { createRequire } from 'node:module'
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const DOCS = join(ROOT, 'docs')
const errors = []
const warnings = []

const rel = (p) => relative(ROOT, p).split('\\').join('/')
const err = (check, message) => errors.push({ check, message })
const warn = (check, message) => warnings.push({ check, message })

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name.startsWith('.')) continue
    const full = join(dir, name)
    if (statSync(full).isDirectory()) walk(full, out)
    else if (name.endsWith('.md') || name.endsWith('.png')) out.push(full)
  }
  return out
}

const allFiles = walk(DOCS).concat([join(ROOT, 'README.md'), join(ROOT, 'AGENTS.md')])
const markdown = allFiles.filter((f) => f.endsWith('.md')).sort()
const read = (f) => readFileSync(f, 'utf8')

/** E1 · 相对链接可解析 */
function checkLinks() {
  for (const file of markdown) {
    const text = read(file)
    for (const [, raw] of text.matchAll(/\]\(([^)\s]+)\)/g)) {
      if (/^(https?:|mailto:|#)/.test(raw)) continue
      const target = decodeURIComponent(raw.split('#')[0].split('?')[0])
      if (target === '') continue
      const full = resolve(dirname(file), target)
      if (!existsSync(full)) err('E1', `${rel(file)} → 链接目标不存在: ${raw}`)
    }
  }
}

const FR_ID = /FR-[A-Z]+-\d+/g
const collectFr = (files) => {
  const set = new Set()
  for (const file of files) for (const [id] of read(file).matchAll(FR_ID)) set.add(id)
  return set
}

/** E2 · FR 编号双向一致（需求 ↔ 任务） */
function checkFrCoverage() {
  const prd = join(DOCS, 'AI-PRD.md')
  const plans = markdown.filter((f) => f.includes(join('docs', 'exec-plans')))
  const inPrd = collectFr([prd])
  const inPlans = collectFr(plans)
  for (const id of [...inPrd].sort()) {
    if (!inPlans.has(id)) err('E2', `${id} 在 AI-PRD 中定义，但没有任何执行计划承接`)
  }
  for (const id of [...inPlans].sort()) {
    if (!inPrd.has(id)) err('E2', `${id} 被计划引用，但 AI-PRD 中没有定义`)
  }
  return { prd: inPrd.size, plans: inPlans.size }
}

/** E3 · mermaid 源码 ↔ 渲染 PNG 成对且不脱节 */
function checkDiagrams() {
  const mermaidRe = /^```mermaid\n([\s\S]*?)^```$/gm
  for (const file of markdown) {
    const text = read(file)
    if (!mermaidRe.test(text)) continue
    const png = file.replace(/\.md$/, '.png')
    if (!existsSync(png)) {
      err('E3', `${rel(file)} 含 mermaid，但缺少渲染图 ${rel(png)}`)
      continue
    }
    if (statSync(png).mtimeMs < statSync(file).mtimeMs) {
      err('E3', `${rel(png)} 早于源码 ${rel(file)}——图可能与源码脱节，请重渲染`)
    }
  }
}

/** E4 · exec-plans 的验收用例 JSON 块 */
function checkPlanJson() {
  const plans = markdown.filter((f) => f.includes(join('docs', 'exec-plans')))
  for (const file of plans) {
    const blocks = [...read(file).matchAll(/^```json\n([\s\S]*?)^```$/gm)]
    blocks.forEach(([, body], i) => {
      let parsed
      try {
        parsed = JSON.parse(body)
      } catch (e) {
        err('E4', `${rel(file)} 第 ${i + 1} 个 json 块解析失败: ${e.message}`)
        return
      }
      if (typeof parsed.description !== 'string' || parsed.description.trim() === '') {
        err('E4', `${rel(file)} 第 ${i + 1} 个用例缺 description`)
      }
      if (typeof parsed.passes !== 'boolean') {
        err('E4', `${rel(file)} 第 ${i + 1} 个用例缺布尔 passes（未验证的用例必须显式为 false）`)
      }
      if (parsed.steps !== undefined) {
        const ok =
          Array.isArray(parsed.steps) &&
          parsed.steps.length > 0 &&
          parsed.steps.every((s) => typeof s === 'string' && s.trim() !== '')
        if (!ok) err('E4', `${rel(file)} 第 ${i + 1} 个用例的 steps 必须是非空字符串数组`)
      }
    })
  }
}

/** W1 · 文档开头的 AI-STD 标注（仓库文档约定） */
function checkStandardAnnotation() {
  for (const file of markdown) {
    const head = read(file).split('\n').slice(0, 12).join('\n')
    if (!head.includes('AI-STD')) warn('W1', `${rel(file)} 未在开头标注所对齐的 AI-STD 条款`)
  }
}

/** W2 · prettier 格式（按仓库现行 CRLF 行尾校验） */
async function checkPrettier() {
  let prettier
  try {
    prettier = createRequire(import.meta.url)('prettier')
  } catch {
    warn('W2', '未找到 prettier（先 `pnpm install`）——本次跳过格式检查')
    return
  }
  for (const file of markdown.concat([join(ROOT, 'package.json')])) {
    const source = read(file)
    // 必须用 resolveConfig 读仓库 .prettierrc（check 不会自动加载配置）；
    // 再覆盖 endOfLine：.prettierrc 写的是 lf，但仓库现存文件全是 CRLF，
    // 按现状校验，避免出现「全仓行尾翻新」的无关改动。全仓切 LF 时应删掉该覆盖。
    const config = (await prettier.resolveConfig(file)) ?? {}
    const ok = await prettier.check(source, { ...config, filepath: file, endOfLine: 'crlf' })
    if (!ok) warn('W2', `${rel(file)} 不符合 prettier 格式（pnpm run format）`)
  }
}

checkLinks()
const fr = checkFrCoverage()
checkDiagrams()
checkPlanJson()
checkStandardAnnotation()
await checkPrettier()

console.log('docs-check · 文档一致性与合规自检（AI-STD-006）')
console.log(
  `扫描 ${markdown.length} 份 markdown · FR ${fr.prd}/${fr.plans}（AI-PRD/计划）· E3 图表成对`,
)
for (const { check, message } of errors) console.log(`[error] ${check}  ${message}`)
for (const { check, message } of warnings) console.log(`[warn ] ${check}  ${message}`)
console.log(`\n错误 ${errors.length} · 告警 ${warnings.length}`)
if (errors.length > 0) {
  console.log('存在硬性不一致：修掉 error 再继续（告警可按需清理）。')
  process.exit(1)
}
console.log('通过：文档与需求、任务、图表一致。')
