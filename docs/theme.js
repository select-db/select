const stored = localStorage.getItem('doc-theme');
const system = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
document.documentElement.setAttribute('data-theme', stored || system);

window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', e => {
  if (!localStorage.getItem('doc-theme')) {
    document.documentElement.setAttribute('data-theme', e.matches ? 'dark' : 'light');
  }
});

const isMac = navigator.userAgentData?.platform === 'macOS' || /Mac/.test(navigator.userAgent);
document.querySelectorAll('.search-shortcut').forEach(el => {
  el.textContent = isMac ? '⌘K' : 'Ctrl+K';
});

function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme');
  const next = current === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('doc-theme', next);
}
