// Update URL hash on scroll based on visible heading
const headings = () => document.querySelectorAll('.content h1[id], .content h2[id], .content h3[id]')

let ticking = false
document.addEventListener('scroll', () => {
  if (ticking) return
  ticking = true
  requestAnimationFrame(() => {
    const items = headings()
    let current = ''
    for (const h of items) {
      if (h.getBoundingClientRect().top <= 80) current = h.id
    }
    if (current && location.hash !== '#' + current) {
      history.replaceState(null, '', '#' + current)
    }
    ticking = false
  })
})

// Click anchor to copy link instead of navigating
document.addEventListener('click', (e) => {
  const anchor = e.target.closest('.anchor')
  if (!anchor) return
  e.preventDefault()
  const url = location.origin + location.pathname + anchor.getAttribute('href')
  navigator.clipboard.writeText(url).then(() => {
    const toast = document.createElement('div')
    toast.className = 'copy-toast'
    toast.textContent = 'Link copied'
    document.body.appendChild(toast)
    requestAnimationFrame(() => toast.classList.add('visible'))
    setTimeout(() => {
      toast.classList.remove('visible')
      setTimeout(() => toast.remove(), 150)
    }, 1500)
  })
  history.replaceState(null, '', anchor.getAttribute('href'))
})
