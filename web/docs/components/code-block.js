class DocCodeBlock extends HTMLElement {
  connectedCallback() {
    const pre = this.querySelector('pre')
    if (!pre) return

    const btn = document.createElement('button')
    btn.className = 'copy-btn'
    btn.textContent = 'Copy'
    btn.addEventListener('click', () => {
      const code = pre.querySelector('code') || pre
      navigator.clipboard.writeText(code.textContent).then(() => {
        btn.textContent = 'Copied'
        setTimeout(() => { btn.textContent = 'Copy' }, 1500)
      })
    })

    this.style.position = 'relative'
    btn.style.cssText = `position:absolute;top:var(--space-xs);right:var(--space-xs);font-size:var(--fs-xs);padding:var(--space-xxs) var(--space-xs);background:var(--gray-300);border:var(--border);border-radius:var(--br-sm);cursor:pointer;color:var(--gray-800);`
    this.appendChild(btn)
  }
}

customElements.define('doc-code-block', DocCodeBlock)
