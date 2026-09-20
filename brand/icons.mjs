// Derives the icon variants that other places need from app/build/icon-default.png.
//
//   ./dev.sh app icons
//
// Two outputs, one source, because both are the same mark under a different
// frame and a second drawing of it is a second thing to keep in step:
//
//   app/build/appicon.mac-default.png  the macOS .icns source
//   brand/github-avatar.png            the GitHub organisation avatar
//
// The macOS frame is not ours to choose. A legacy .icns is drawn exactly as
// authored, and the convention -- measured off shipping icons, since Apple
// stopped publishing the template -- is an 824px body centred on a 1024 canvas
// with a soft shadow. Windows and Linux want the artwork to fill the canvas
// instead, which is what icon-default.png already is, so they read it directly.
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { copyFileSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';

// playwright belongs to app/frontend, where the screenshot specs run and where
// the task runs this, so resolve from there rather than asking for a second copy.
const require = createRequire(path.join(process.cwd(), 'package.json'));
const { chromium } = require('playwright');

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');
const src = path.join(root, 'app', 'build', 'icon-default.png');

const CANVAS = 1024;
const BODY = 824;
// The tile colour, flattened behind the mark to square off its corners. GitHub
// crops an avatar square and rounds it itself; a radius baked in is drawn twice.
const TILE = 'rgb(172, 45, 49)';

const browser = await chromium.launch();
const page = await browser.newPage();
const out = await page.evaluate(
	async ([b64, CANVAS, BODY, TILE]) => {
		const img = new Image();
		img.src = 'data:image/png;base64,' + b64;
		await img.decode();

		const draw = (paint) => {
			const c = document.createElement('canvas');
			c.width = CANVAS;
			c.height = CANVAS;
			paint(c.getContext('2d'));
			return c;
		};
		const encode = async (c) => {
			const blob = await new Promise((r) => c.toBlob(r, 'image/png'));
			const buf = new Uint8Array(await blob.arrayBuffer());
			let s = '';
			for (const byte of buf) s += String.fromCharCode(byte);
			return btoa(s);
		};

		// Where the artwork actually starts, so the body lands at BODY whatever
		// margin the source was drawn with.
		const probe = draw((ctx) => ctx.drawImage(img, 0, 0));
		const d = probe.getContext('2d', { willReadFrequently: true }).getImageData(0, 0, CANVAS, CANVAS).data;
		let x0 = CANVAS, x1 = -1;
		for (let y = 0; y < CANVAS; y++) {
			for (let x = 0; x < CANVAS; x++) {
				if (d[(y * CANVAS + x) * 4 + 3] > 200) {
					if (x < x0) x0 = x;
					if (x > x1) x1 = x;
				}
			}
		}
		const scale = BODY / (x1 - x0 + 1);
		const margin = (CANVAS - BODY) / 2;

		const mac = draw((ctx) => {
			ctx.imageSmoothingQuality = 'high';
			// Reaches ~15px to the sides and ~25px below the body, which is where
			// both reference icons put it.
			ctx.shadowColor = 'rgba(0,0,0,.22)';
			ctx.shadowBlur = 21;
			ctx.shadowOffsetY = 10;
			ctx.drawImage(img, margin - x0 * scale, margin - x0 * scale, CANVAS * scale, CANVAS * scale);
		});
		const avatar = draw((ctx) => {
			ctx.fillStyle = TILE;
			ctx.fillRect(0, 0, CANVAS, CANVAS);
			ctx.drawImage(img, 0, 0);
		});
		return { mac: await encode(mac), avatar: await encode(avatar) };
	},
	[readFileSync(src).toString('base64'), CANVAS, BODY, TILE]
);
await browser.close();

const macDefault = path.join(root, 'app', 'build', 'appicon.mac-default.png');
writeFileSync(macDefault, Buffer.from(out.mac, 'base64'));
// The working copy the build reads, kept beside its default exactly as
// appicon.png is kept beside appicon-default.png.
copyFileSync(macDefault, path.join(root, 'app', 'build', 'appicon.mac.png'));
writeFileSync(path.join(root, 'brand', 'github-avatar.png'), Buffer.from(out.avatar, 'base64'));

console.log('wrote app/build/appicon.mac{,-default}.png and brand/github-avatar.png');
