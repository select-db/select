/**
 * Workspace logo helpers.
 *
 * A logo is stored as the bare base64 of a 128x128 PNG — no `data:` prefix, so a
 * stored value can never bring its own media type into an `<img src>`. This
 * module owns both ends of that convention: turning a picked file into the
 * base64 the backend expects, and composing the data URL for rendering.
 *
 * Normalizing here is a convenience, not a security boundary: the backend
 * re-decodes and re-encodes whatever it receives (see backend/internal/workspace/
 * logo.go). What it buys us is that a 12 MB phone photo never leaves the machine,
 * and that the canvas round trip drops EXIF and colour profiles on the way.
 */

export const LOGO_SIZE = 128;

/** Formats the browser can decode for us. SVG is deliberately absent. */
export const LOGO_ACCEPT = 'image/png,image/jpeg,image/webp';

const ALLOWED_TYPES = new Set(['image/png', 'image/jpeg', 'image/webp']);

/** Refuses an obviously oversized pick before spending memory decoding it. */
const MAX_FILE_BYTES = 5 * 1024 * 1024;

/** Composes the src for an `<img>`, or null when the workspace has no logo. */
export function logoSrc(base64: string | null | undefined): string | null {
	if (!base64) return null;
	return `data:image/png;base64,${base64}`;
}

/**
 * Decodes a picked file and re-encodes it as a 128x128 PNG, returning bare
 * base64. The image is scaled to cover the square and centre-cropped, so a
 * non-square logo keeps its aspect ratio instead of being stretched.
 */
export async function fileToLogoBase64(file: File): Promise<string> {
	if (!ALLOWED_TYPES.has(file.type)) {
		throw new Error('Logo must be a PNG, JPEG or WebP image');
	}
	if (file.size > MAX_FILE_BYTES) {
		throw new Error('Logo must be smaller than 5 MB');
	}

	let bitmap: ImageBitmap;
	try {
		bitmap = await createImageBitmap(file);
	} catch {
		throw new Error('Could not read that image');
	}

	try {
		const canvas = document.createElement('canvas');
		canvas.width = LOGO_SIZE;
		canvas.height = LOGO_SIZE;

		const ctx = canvas.getContext('2d');
		if (!ctx) throw new Error('Could not read that image');

		// Cover: scale by the larger ratio, then centre what overflows.
		const scale = Math.max(LOGO_SIZE / bitmap.width, LOGO_SIZE / bitmap.height);
		const width = bitmap.width * scale;
		const height = bitmap.height * scale;
		ctx.imageSmoothingQuality = 'high';
		ctx.drawImage(bitmap, (LOGO_SIZE - width) / 2, (LOGO_SIZE - height) / 2, width, height);

		const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'));
		if (!blob) throw new Error('Could not read that image');

		return bytesToBase64(new Uint8Array(await blob.arrayBuffer()));
	} finally {
		bitmap.close();
	}
}

function bytesToBase64(bytes: Uint8Array): string {
	// Chunked so a large image cannot blow the argument limit of String.fromCharCode.
	const chunkSize = 0x8000;
	let binary = '';
	for (let i = 0; i < bytes.length; i += chunkSize) {
		binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
	}
	return btoa(binary);
}
