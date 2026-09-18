class DocToc extends HTMLElement {
  connectedCallback() {
    const headings = document.querySelectorAll('.content h1[id], .content h2[id], .content h3[id]')
    if (headings.length < 2) return

    const nav = document.createElement('nav')
    nav.innerHTML = ''
    nav.setAttribute('aria-label', 'On this page')

    // A real heading, not a ::before: it names the rail for a screen reader as
    // well as for a reader, and it is the same eyebrow the rest of the site
    // puts over a column.
    const label = document.createElement('p')
    label.className = 'toc-label'
    label.textContent = 'On this page'
    nav.appendChild(label)

    const ul = document.createElement('ul')

    headings.forEach(h => {
      const li = document.createElement('li')
      li.className = 'toc-' + h.tagName.toLowerCase()
      const a = document.createElement('a')
      a.href = '#' + h.id
      a.textContent = h.textContent.replace(/\s*#\s*$/, '')
      li.appendChild(a)
      ul.appendChild(li)
    })

    nav.appendChild(ul)
    this.appendChild(nav)

    // Highlight active heading on scroll
    const main = document.querySelector('main')
    if (!main) return

    const links = ul.querySelectorAll('a')
    let ticking = false

    main.addEventListener('scroll', () => {
      if (ticking) return
      ticking = true
      requestAnimationFrame(() => {
        let current = ''
        headings.forEach(h => {
          if (h.getBoundingClientRect().top <= 100) current = h.id
        })
        links.forEach(a => {
          a.classList.toggle('active', a.getAttribute('href') === '#' + current)
        })
        ticking = false
      })
    })
  }
}

customElements.define('doc-toc', DocToc)
