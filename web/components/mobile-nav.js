// Close sidebar on link click (mobile)
document.querySelector('doc-sidebar')?.addEventListener('click', (e) => {
  if (e.target.closest('a')) {
    document.body.classList.remove('sidebar-open')
  }
})
