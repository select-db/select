import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],

	build: {
		// The bundle ships inside the binary and is read from disk, never over a
		// network, so vite's 500 kB advice does not apply — the editor and the
		// terminal alone are several times that. The limit is raised rather than
		// removed so a genuine step change still gets flagged.
		chunkSizeWarningLimit: 8000
	}
});
