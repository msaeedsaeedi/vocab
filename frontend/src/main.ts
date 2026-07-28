import './style.css'
import './app.css'

import {
  GetDueWord,
  RecordFeedback,
  GetStats,
  SaveWindowPosition,
  HideToTray,
} from '../wailsjs/go/main/App'
import {
  WindowSetPosition,
  WindowGetPosition,
} from '../wailsjs/runtime/runtime'

interface WordCard {
  id: number
  text: string
  definition: string
  example: string
  box: number
}

interface Stats {
  total: number
  due_today: number
}

let currentWord: WordCard | null = null

let isDragging = false
let dragStartX = 0
let dragStartY = 0
let winStartX = 0
let winStartY = 0

async function loadWord() {
  const word = (await GetDueWord()) as WordCard | null
  currentWord = word

  const wordEl = document.getElementById('word')!
  const defEl = document.getElementById('definition')!
  const exampleEl = document.getElementById('example')!
  const btns = document.getElementById('buttons')!
  const doneEl = document.getElementById('done')!
  const statsEl = document.getElementById('stats')!
  const emptyEl = document.getElementById('empty')!

  wordEl.style.display = 'none'
  defEl.style.display = 'none'
  exampleEl.style.display = 'none'
  btns.style.display = 'none'
  doneEl.style.display = 'none'
  emptyEl.style.display = 'none'

  if (!word) {
    doneEl.style.display = 'block'
    const stats = (await GetStats()) as Stats
    if (stats.total > 0) {
      doneEl.textContent = 'All caught up! Come back tomorrow.'
    } else {
      emptyEl.style.display = 'block'
    }
    statsEl.textContent = ''
    return
  }

  wordEl.textContent = word.text
  wordEl.style.display = 'block'

  if (word.definition) {
    defEl.textContent = word.definition
    defEl.style.display = 'block'
  }
  if (word.example) {
    exampleEl.textContent = `"${word.example}"`
    exampleEl.style.display = 'block'
  }

  btns.style.display = 'flex'

  const stats = (await GetStats()) as Stats
  statsEl.textContent = `${stats.total} words  ·  ${stats.due_today} due`
}

async function answer(knewIt: boolean) {
  if (!currentWord) return
  await RecordFeedback(currentWord.id, knewIt)
  currentWord = null
  await loadWord()
}

function setupDrag() {
  const region = document.getElementById('drag-region')!

  region.addEventListener('mousedown', async (e) => {
    isDragging = true
    dragStartX = e.clientX
    dragStartY = e.clientY
    try {
      const pos = await WindowGetPosition()
      winStartX = pos.x
      winStartY = pos.y
    } catch {
      isDragging = false
    }
  })

  document.addEventListener('mousemove', (e) => {
    if (!isDragging) return
    const dx = e.clientX - dragStartX
    const dy = e.clientY - dragStartY
    WindowSetPosition(winStartX + dx, winStartY + dy)
  })

  document.addEventListener('mouseup', async () => {
    if (!isDragging) return
    isDragging = false
    try {
      const pos = await WindowGetPosition()
      SaveWindowPosition(pos.x, pos.y)
    } catch {
      // silent
    }
  })
}

function setupTrayClose() {
  document.getElementById('tray-close')!.addEventListener('click', async () => {
    await HideToTray()
  })
}

function setupKeyboard() {
  document.addEventListener('keydown', async (e) => {
    if (e.key === 'Escape') {
      await HideToTray()
    }
  })
}

document.addEventListener('DOMContentLoaded', () => {
  setupDrag()
  setupTrayClose()
  setupKeyboard()
  loadWord()
})

window.loadWord = loadWord
window.answer = answer

declare global {
  interface Window {
    loadWord: () => Promise<void>
    answer: (knewIt: boolean) => Promise<void>
  }
}
