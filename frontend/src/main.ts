import './style.css'
import './app.css'

import { GetDueWord, RecordFeedback, GetStats } from '../wailsjs/go/main/App'

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

async function loadWord() {
  const word = await GetDueWord() as WordCard | null
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
    const stats = await GetStats() as Stats
    if (stats.total > 0) {
      doneEl.textContent = 'No words due today! Come back tomorrow.'
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

  const stats = await GetStats() as Stats
  statsEl.textContent = `${stats.total} words  ·  ${stats.due_today} due`
}

async function answer(knewIt: boolean) {
  if (!currentWord) return

  await RecordFeedback(currentWord.id, knewIt)
  currentWord = null
  await loadWord()
}

window.loadWord = loadWord
window.answer = answer

document.addEventListener('DOMContentLoaded', loadWord)

declare global {
  interface Window {
    loadWord: () => Promise<void>
    answer: (knewIt: boolean) => Promise<void>
  }
}
