class DocSearch extends HTMLElement {
  constructor() {
    super()
    this.index = null
    this.dialog = null
    this.selected = -1
  }

  connectedCallback() {
    this.dialog = document.createElement('dialog')
    this.dialog.className = 'search-dialog'
    this.dialog.innerHTML = `
      <div class="search-box">
        <svg class="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
             stroke-linecap="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><line x1="16.5" y1="16.5" x2="21" y2="21"/></svg>
        <input type="text" placeholder="Search docs..." autofocus>
      </div>
      <ul class="search-results scrollable"></ul>
      <div class="search-hints">
        <span><kbd>&uarr;</kbd><kbd>&darr;</kbd> Select</span>
        <span><kbd>&crarr;</kbd> Open</span>
        <span><kbd>Esc</kbd> Close</span>
      </div>
    `
    this.appendChild(this.dialog)

    this.input = this.dialog.querySelector('input')
    this.resultsList = this.dialog.querySelector('.search-results')

    this.input.addEventListener('input', () => {
      this.selected = -1
      this.search(this.input.value)
    })

    this.resultsList.addEventListener('click', (e) => {
      if (e.target.closest('a')) this.close()
    })

    this.input.addEventListener('keydown', (e) => {
      const items = this.resultsList.querySelectorAll('a')
      if (!items.length) return
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        this.selected = Math.min(this.selected + 1, items.length - 1)
        this.highlight(items)
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        this.selected = Math.max(this.selected - 1, -1)
        this.highlight(items)
      } else if (e.key === 'Enter') {
        e.preventDefault()
        const target = this.selected >= 0 ? this.selected : 0
        if (items[target]) items[target].click()
      }
    })

    this.dialog.addEventListener('click', (e) => {
      if (e.target === this.dialog) this.close()
    })

    document.addEventListener('keydown', (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        this.open()
      }
    })
  }

  highlight(items) {
    items.forEach((a, i) => {
      a.parentElement.classList.toggle('active', i === this.selected)
    })
    if (this.selected >= 0) {
      items[this.selected].scrollIntoView({ block: 'nearest' })
    }
  }

  async open() {
    if (!this.index) {
      const res = await fetch('/search-index.json')
      this.index = await res.json()
    }
    this.dialog.showModal()
    this.input.value = ''
    this.resultsList.innerHTML = ''
    this.selected = -1
    this.input.focus()
  }

  close() {
    this.dialog.close()
  }

  search(query) {
    if (!this.index || !query.trim()) {
      this.resultsList.innerHTML = ''
      return
    }

    const q = query.toLowerCase()
    const matches = this.index.filter(entry =>
      entry.title.toLowerCase().includes(q) ||
      entry.body.toLowerCase().includes(q)
    ).slice(0, 10)

    if (!matches.length) {
      this.resultsList.innerHTML = '<li class="no-results">No results</li>'
      return
    }

    this.resultsList.innerHTML = matches.map(m => {
      const title = this.highlightText(m.title, q)
      const excerpt = this.getExcerpt(m.body, q)
      // The ancestry is what tells two identically named headings apart, so it
      // is a line of its own rather than a prefix on the title.
      const path = m.path ? `<div class="result-path">${this.highlightText(m.path, q)}</div>` : ''
      return `<li><a href="${m.href}">
        <span class="result-hash" aria-hidden="true">#</span>
        <span class="result-text">
          <div class="result-title">${title}</div>
          ${path}
          <div class="result-excerpt">${excerpt}</div>
        </span>
      </a></li>`
    }).join('')
  }

  highlightText(text, query) {
    const i = text.toLowerCase().indexOf(query)
    if (i === -1) return this.esc(text)
    return this.esc(text.slice(0, i)) +
      '<mark>' + this.esc(text.slice(i, i + query.length)) + '</mark>' +
      this.esc(text.slice(i + query.length))
  }

  getExcerpt(body, query) {
    const i = body.toLowerCase().indexOf(query)
    if (i === -1) return this.esc(body.slice(0, 80)) + '...'
    const start = Math.max(0, i - 40)
    const end = Math.min(body.length, i + query.length + 40)
    const before = (start > 0 ? '...' : '') + this.esc(body.slice(start, i))
    const match = '<mark>' + this.esc(body.slice(i, i + query.length)) + '</mark>'
    const after = this.esc(body.slice(i + query.length, end)) + (end < body.length ? '...' : '')
    return before + match + after
  }

  esc(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  }
}

customElements.define('doc-search', DocSearch)
