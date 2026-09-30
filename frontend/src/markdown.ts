// 轻量 Markdown 渲染器。
//
// 只覆盖模型日报实际使用的语法子集：`#`~`######` 标题、无序/有序列表、管道表格、
// `**加粗**`、`` `行内代码` ``、`[链接](url)` 与普通段落（其余行按段落降级处理，
// 不会丢内容）。先做 HTML 转义再套用标签，因此配合 v-html 使用时不存在注入问题。
//
// 选型说明：仓库依赖刻意保持极简（vue / echarts / lucide），且渲染需求有明确边界，
// 故不引入 marked 等外部解析器；若日后需要完整 Markdown 支持（嵌套列表、图片、
// 脚注等），可整体替换本模块的实现，调用方无需改动。

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function inline(value: string): string {
  return escapeHtml(value)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer noopener">$1</a>')
}

const HEADING = /^(#{1,6})\s+(.*)$/
const UNORDERED = /^[-*]\s+(.*)$/
const ORDERED = /^\d+[.)]\s+(.*)$/
const TABLE_ROW = /^\|(.+)\|\s*$/
const TABLE_SEPARATOR = /^\|[\s:|-]+\|\s*$/

export function renderMarkdown(source: string): string {
  const lines = source.replace(/\r\n?/g, '\n').split('\n')
  const html: string[] = []
  let list: 'ul' | 'ol' | null = null
  let paragraph: string[] = []
  let table: string[][] = []

  const closeList = () => {
    if (list) {
      html.push(`</${list}>`)
      list = null
    }
  }
  const closeParagraph = () => {
    if (paragraph.length) {
      html.push(`<p>${inline(paragraph.join(' '))}</p>`)
      paragraph = []
    }
  }
  const closeTable = () => {
    if (!table.length) return
    const [head, ...body] = table
    const headCells = head.map((cell) => `<th>${inline(cell)}</th>`).join('')
    const bodyRows = body
      .map((row) => `<tr>${row.map((cell) => `<td>${inline(cell)}</td>`).join('')}</tr>`)
      .join('')
    html.push(`<table><thead><tr>${headCells}</tr></thead><tbody>${bodyRows}</tbody></table>`)
    table = []
  }

  for (const raw of lines) {
    const line = raw.trim()

    if (TABLE_ROW.test(line)) {
      closeParagraph()
      closeList()
      if (!TABLE_SEPARATOR.test(line)) {
        table.push(line.slice(1, -1).split('|').map((cell) => cell.trim()))
      }
      continue
    }
    closeTable()

    if (!line) {
      closeParagraph()
      closeList()
      continue
    }

    const heading = line.match(HEADING)
    if (heading) {
      closeParagraph()
      closeList()
      // # -> h3：报告内标题不与页面 h1/h2 抢层级
      const level = Math.min(heading[1].length + 2, 6)
      html.push(`<h${level}>${inline(heading[2])}</h${level}>`)
      continue
    }

    const unordered = line.match(UNORDERED)
    if (unordered) {
      closeParagraph()
      if (list !== 'ul') {
        closeList()
        html.push('<ul>')
        list = 'ul'
      }
      html.push(`<li>${inline(unordered[1])}</li>`)
      continue
    }

    const ordered = line.match(ORDERED)
    if (ordered) {
      closeParagraph()
      if (list !== 'ol') {
        closeList()
        html.push('<ol>')
        list = 'ol'
      }
      html.push(`<li>${inline(ordered[1])}</li>`)
      continue
    }

    closeList()
    paragraph.push(line)
  }

  closeParagraph()
  closeList()
  closeTable()
  return html.join('')
}
