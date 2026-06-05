import { getCSSVariable } from '$lib/utils/getCSSVariable';
import { hslToHex } from '$lib/utils/hslToHex';
import type { ITheme } from '@xterm/xterm';

export const getTerminalTheme = (): ITheme => {
	const surfaceLight = hslToHex(getCSSVariable('--gray-100'));
	const contrast = hslToHex(getCSSVariable('--gray-800'));
	const contrastDimmed = hslToHex(getCSSVariable('--gray-800'));
	const contrastSurface = hslToHex(getCSSVariable('--gray-600'));
	const red = hslToHex(getCSSVariable('--red'));
	const blue = hslToHex(getCSSVariable('--blue'));
	const green = hslToHex(getCSSVariable('--green'));
	const yellow = hslToHex(getCSSVariable('--yellow'));
	const orange = hslToHex(getCSSVariable('--orange'));
	const purple = hslToHex(getCSSVariable('--purple'));

	return {
		background: surfaceLight,
		foreground: contrastDimmed,
		cursor: contrastDimmed,
		cursorAccent: surfaceLight,
		selectionBackground: contrastSurface,

		black: surfaceLight,
		red,
		green,
		yellow,
		blue,
		magenta: purple,
		cyan: blue,
		white: contrastDimmed,

		brightBlack: contrastDimmed,
		brightRed: red,
		brightGreen: green,
		brightYellow: orange,
		brightBlue: blue,
		brightMagenta: purple,
		brightCyan: blue,
		brightWhite: contrast
	};
};
