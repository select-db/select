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

// Keycaps are authored in the Mac spelling. Off a Mac the same chord is pressed
// with different keys, so the caps say so.
if (!isMac) {
  const label = { cmd: 'Ctrl', alt: 'Alt' };
  document.querySelectorAll('kbd[data-mod]').forEach(el => {
    const next = label[el.dataset.mod];
    if (next) el.textContent = next;
  });
}

// Screenshots come in a light cut and a dark cut, one word apart. <picture>
// art-directs on its own, but its media query only ever sees the system
// preference, never a choice made with the toggle -- so the toggle renames the
// files itself, the same way the marketing page does.
function paintShots(theme) {
  document.querySelectorAll('picture[data-shot] source, picture[data-shot] img').forEach(el => {
    const attr = el.tagName === 'IMG' ? 'src' : 'srcset';
    const next = el[attr].replace(/\.(light|dark)\.png/, '.' + theme + '.png');
    if (next !== el[attr]) el[attr] = next;
  });
}

paintShots(document.documentElement.getAttribute('data-theme'));

function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme');
  const next = current === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('doc-theme', next);
  paintShots(next);
}
