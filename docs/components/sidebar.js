class DocSidebar extends HTMLElement {
  connectedCallback() {
    const stored = localStorage.getItem('doc-sidebar-state')
    if (stored) {
      const openSections = JSON.parse(stored)
      this.querySelectorAll('details').forEach(d => {
        const key = d.querySelector('summary').textContent.trim()
        if (openSections.includes(key)) d.open = true
      })
    }

    this.addEventListener('toggle', (e) => {
      if (e.target.tagName === 'DETAILS') this.save()
    }, true)
  }

  save() {
    const open = []
    this.querySelectorAll('details[open]').forEach(d => {
      open.push(d.querySelector('summary').textContent.trim())
    })
    localStorage.setItem('doc-sidebar-state', JSON.stringify(open))
  }
}

customElements.define('doc-sidebar', DocSidebar)
