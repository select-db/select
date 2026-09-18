/**
 * A logo is stored as the bare base64 of a 128x128 PNG — no `data:` prefix, so a
 * stored value can never bring its own media type into an `<img src>`. This
 * module owns both ends of that convention.
 *
 * Normalizing here is a convenience, not a security boundary: the backend
 * re-decodes and re-encodes whatever it receives (backend/internal/workspace/
 * logo.go). It buys us a 12 MB phone photo never leaving the machine.
 */

export const LOGO_SIZE = 128;

/** SVG is deliberately absent. */
export const LOGO_ACCEPT = 'image/png,image/jpeg,image/webp';

const ALLOWED_TYPES = new Set(['image/png', 'image/jpeg', 'image/webp']);
const MAX_FILE_BYTES = 5 * 1024 * 1024;

export function logoSrc(base64: string | null | undefined): string | null {
	if (!base64) return null;
	return `data:image/png;base64,${base64}`;
}

/**
 * Decodes a picked file and re-encodes it as a 128x128 PNG, returning bare
 * base64. Scaled to cover and centre-cropped, so a non-square logo keeps its
 * aspect ratio instead of being stretched.
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
	// Chunked: String.fromCharCode has an argument limit.
	const chunkSize = 0x8000;
	let binary = '';
	for (let i = 0; i < bytes.length; i += chunkSize) {
		binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
	}
	return btoa(binary);
}
