// Renders card.html into web/og.png, the image every link preview shows.
//
//   wails3 task og          (from app/, which installs and runs it)
//
// Chromium already drives the screenshot specs, and this card is the same kind
// of artifact they produce: rendered once, committed, served as a file. Run it
// after editing card.html and commit what changed.
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

// playwright belongs to app/frontend, where the screenshot specs run and where
// the task runs this, so resolve from there rather than asking for a second copy.
const require = createRequire(path.join(process.cwd(), 'package.json'));
const { chromium } = require('playwright');

const here = path.dirname(fileURLToPath(import.meta.url));
const out = path.join(here, '..', 'og.png');

const browser = await chromium.launch();
const page = await browser.newPage({
	viewport: { width: 1200, height: 630 },
	// The card is authored at its final pixel size: anything above 1 gives a
	// 2400px image every platform scales back down.
	deviceScaleFactor: 1
});
await page.goto('file://' + path.join(here, 'card.html'));
await page.waitForLoadState('networkidle');
await page.screenshot({ path: out });
await browser.close();

console.log('wrote ' + path.relative(process.cwd(), out));
