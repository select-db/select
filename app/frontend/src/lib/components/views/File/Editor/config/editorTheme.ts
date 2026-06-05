import { getCSSVariable } from '$lib/utils/getCSSVariable';
import { hslToHex } from '$lib/utils/hslToHex';

export const EDITOR_THEME_NAME = 'selectdb-theme';

/**
 * Unified editor theme containing token rules for ALL languages.
 * Monaco themes are global - this single theme handles sql, dotenv, etc.
 */
export const getEditorTheme = () => {
	const isDark = document.documentElement.getAttribute('data-theme') === 'dark';

	const gray0 = hslToHex(getCSSVariable('--gray-0'));
	const gray200 = hslToHex(getCSSVariable('--gray-200'));
	const gray300 = hslToHex(getCSSVariable('--gray-300'));
	const gray400 = hslToHex(getCSSVariable('--gray-400'));
	const gray600 = hslToHex(getCSSVariable('--gray-600'));
	const gray700 = hslToHex(getCSSVariable('--gray-700'));
	const gray1000 = hslToHex(getCSSVariable('--gray-1000'));

	const red = hslToHex(getCSSVariable('--red'));
	const blue = hslToHex(getCSSVariable('--blue'));
	const green = hslToHex(getCSSVariable('--green'));
	const orange = hslToHex(getCSSVariable('--orange'));
	const yellow = hslToHex(getCSSVariable('--yellow'));
	const purple = hslToHex(getCSSVariable('--purple'));

	return {
		base: isDark ? 'vs-dark' : 'vs',
		inherit: true,
		rules: [
			// ===== CSS =====
			{ token: 'keyword.css', foreground: red },
			{ token: 'tag.css', foreground: red },
			{ token: 'attribute.name.css', foreground: blue },
			{ token: 'attribute.value.css', foreground: gray1000 },
			{ token: 'attribute.value.number.css', foreground: gray1000 },
			{ token: 'attribute.value.unit.css', foreground: gray1000 },
			{ token: 'attribute.value.hex.css', foreground: gray1000 },
			{ token: 'string.css', foreground: purple },
			{ token: 'comment.css', foreground: gray700, fontStyle: 'italic' },
			{ token: 'delimiter.css', foreground: gray1000 },
			{ token: 'delimiter.bracket.css', foreground: gray1000 },
			{ token: 'delimiter.parenthesis.css', foreground: gray700 },

			// ===== HTML =====
			{ token: 'metatag.html', foreground: red },
			{ token: 'metatag.content.html', foreground: gray1000 },
			{ token: 'tag.html', foreground: red },
			{ token: 'attribute.name.html', foreground: blue },
			{ token: 'attribute.value.html', foreground: purple },
			{ token: 'delimiter.html', foreground: gray700 },
			{ token: 'comment.html', foreground: gray700, fontStyle: 'italic' },
			{ token: 'comment.content.html', foreground: gray700, fontStyle: 'italic' },

			// ===== TypeScript =====
			{ token: 'keyword.ts', foreground: red },
			{ token: 'keyword.other.ts', foreground: gray700 },
			{ token: 'identifier.ts', foreground: gray1000 },
			{ token: 'type.identifier.ts', foreground: blue },
			{ token: 'number.ts', foreground: orange },
			{ token: 'number.float.ts', foreground: orange },
			{ token: 'number.hex.ts', foreground: orange },
			{ token: 'number.octal.ts', foreground: orange },
			{ token: 'number.binary.ts', foreground: orange },
			{ token: 'string.ts', foreground: purple },
			{ token: 'string.escape.ts', foreground: orange },
			{ token: 'string.escape.invalid.ts', foreground: red },
			{ token: 'string.invalid.ts', foreground: red },
			{ token: 'regexp.ts', foreground: purple },
			{ token: 'regexp.escape.ts', foreground: orange },
			{ token: 'regexp.escape.control.ts', foreground: red },
			{ token: 'regexp.invalid.ts', foreground: red },
			{ token: 'comment.ts', foreground: gray700, fontStyle: 'italic' },
			{ token: 'comment.doc.ts', foreground: gray700, fontStyle: 'italic' },
			{ token: 'delimiter.ts', foreground: gray1000 },
			{ token: 'delimiter.bracket.ts', foreground: gray1000 },

			// ===== JavaScript =====
			{ token: 'keyword.js', foreground: red },
			{ token: 'keyword.other.js', foreground: gray700 },
			{ token: 'identifier.js', foreground: gray1000 },
			{ token: 'type.identifier.js', foreground: blue },
			{ token: 'number.js', foreground: orange },
			{ token: 'number.float.js', foreground: orange },
			{ token: 'number.hex.js', foreground: orange },
			{ token: 'number.octal.js', foreground: orange },
			{ token: 'number.binary.js', foreground: orange },
			{ token: 'string.js', foreground: purple },
			{ token: 'string.escape.js', foreground: orange },
			{ token: 'string.escape.invalid.js', foreground: red },
			{ token: 'string.invalid.js', foreground: red },
			{ token: 'regexp.js', foreground: purple },
			{ token: 'regexp.escape.js', foreground: orange },
			{ token: 'regexp.escape.control.js', foreground: red },
			{ token: 'regexp.invalid.js', foreground: red },
			{ token: 'comment.js', foreground: gray700, fontStyle: 'italic' },
			{ token: 'comment.doc.js', foreground: gray700, fontStyle: 'italic' },
			{ token: 'delimiter.js', foreground: gray1000 },
			{ token: 'delimiter.bracket.js', foreground: gray1000 },

			// ===== SQL =====
			{ token: 'keyword.sql', foreground: red },
			{ token: 'function.sql', foreground: blue },
{ token: 'string.sql', foreground: purple },
			{ token: 'identifier.sql', foreground: gray1000 },
			{ token: 'number.sql', foreground: orange },
			{ token: 'comment.sql', foreground: gray700, fontStyle: 'italic' },
			{ token: 'operator.sql', foreground: red },
			{ token: 'variable.sql', foreground: purple },
			{ token: 'type.sql', foreground: blue },
			{ token: 'constant.sql', foreground: purple },

			// ===== Dotenv =====
			{ token: 'comment', foreground: gray700, fontStyle: 'italic' },
			{ token: 'variable.name', foreground: purple },
			{ token: 'string', foreground: gray1000 },

			// ===== Default =====
			{ token: '', foreground: gray1000 }
		],
		colors: {
			// ===== Editor base =====
			'editor.background': gray0,
			'editor.foreground': gray1000,

			// ===== Cursor =====
			'editorCursor.foreground': gray1000,
			'editorCursor.background': gray0,
			'editorMultiCursor.primary.foreground': gray1000,
			'editorMultiCursor.secondary.foreground': orange,

			// ===== Selection =====
			'editor.selectionBackground': isDark ? gray600 : gray400,
			'editor.selectionForeground': gray1000,
			'editor.inactiveSelectionBackground': gray300,
			'editor.selectionHighlightBackground': `${gray400}80`,
			'editor.selectionHighlightBorder': `${gray600}60`,

			// ===== Word highlight (cursor word) =====
			'editor.wordHighlightBackground': `${gray400}60`,
			'editor.wordHighlightStrongBackground': `${gray400}99`,
			'editor.wordHighlightBorder': `${gray600}60`,
			'editor.wordHighlightStrongBorder': `${gray700}60`,

			// ===== Line =====
			'editor.lineHighlightBackground': `${gray600}30`,
			'editor.lineHighlightBorder': `${gray600}20`,

			// ===== Find / search =====
			'editor.findMatchBackground': `${orange}66`,
			'editor.findMatchBorder': orange,
			'editor.findMatchHighlightBackground': `${orange}33`,
			'editor.findMatchHighlightBorder': `${orange}80`,
			'editor.findRangeHighlightBackground': `${gray400}40`,
			'editor.rangeHighlightBackground': `${gray400}30`,
			'editor.symbolHighlightBackground': `${gray400}40`,

			// ===== Hover =====
			'editor.hoverHighlightBackground': `${gray400}40`,
			'editorHoverWidget.background': gray0,
			'editorHoverWidget.foreground': gray1000,
			'editorHoverWidget.border': gray300,
			'editorHoverWidget.statusBarBackground': gray200,

			// ===== Line numbers =====
			'editorLineNumber.foreground': gray700,
			'editorLineNumber.activeForeground': gray1000,
			'editorLineNumber.dimmedForeground': gray600,

			// ===== Whitespace & indent guides =====
			'editorWhitespace.foreground': gray700,
			'editorIndentGuide.background1': gray0,
			'editorIndentGuide.background2': gray0,
			'editorIndentGuide.background3': gray0,
			'editorIndentGuide.background4': gray0,
			'editorIndentGuide.background5': gray0,
			'editorIndentGuide.background6': gray0,
			'editorIndentGuide.activeBackground1': gray600,
			'editorIndentGuide.activeBackground2': gray600,
			'editorIndentGuide.activeBackground3': gray600,
			'editorIndentGuide.activeBackground4': gray600,
			'editorIndentGuide.activeBackground5': gray600,
			'editorIndentGuide.activeBackground6': gray600,

			// ===== Ruler / gutter =====
			'editorRuler.foreground': gray300,
			'editorGutter.background': gray0,
			'editorCodeLens.foreground': gray700,

			// ===== Overview ruler =====
			'editorOverviewRuler.background': gray0,
			'editorOverviewRuler.border': gray300,
			'editorOverviewRuler.errorForeground': red,
			'editorOverviewRuler.warningForeground': orange,
			'editorOverviewRuler.infoForeground': blue,
			'editorOverviewRuler.findMatchForeground': orange,
			'editorOverviewRuler.selectionHighlightForeground': gray600,
			'editorOverviewRuler.rangeHighlightForeground': gray600,

			// ===== Bracket match =====
			'editorBracketMatch.background': gray400,
			'editorBracketMatch.border': blue,

			// ===== Bracket pair colorization =====
			'editorBracketHighlight.foreground1': blue,
			'editorBracketHighlight.foreground2': purple,
			'editorBracketHighlight.foreground3': orange,
			'editorBracketHighlight.foreground4': blue,
			'editorBracketHighlight.foreground5': purple,
			'editorBracketHighlight.foreground6': orange,
			'editorBracketHighlight.unexpectedBracket.foreground': red,
			'editorBracketPairGuide.background1': `${blue}18`,
			'editorBracketPairGuide.background2': `${purple}18`,
			'editorBracketPairGuide.background3': `${orange}18`,
			'editorBracketPairGuide.background4': `${blue}18`,
			'editorBracketPairGuide.background5': `${purple}18`,
			'editorBracketPairGuide.background6': `${orange}18`,
			'editorBracketPairGuide.activeBackground1': `${blue}35`,
			'editorBracketPairGuide.activeBackground2': `${purple}35`,
			'editorBracketPairGuide.activeBackground3': `${orange}35`,
			'editorBracketPairGuide.activeBackground4': `${blue}35`,
			'editorBracketPairGuide.activeBackground5': `${purple}35`,
			'editorBracketPairGuide.activeBackground6': `${orange}35`,

			// ===== Ghost text (inline completions) =====
			'editorGhostText.foreground': gray600,

			// ===== Unnecessary code (dimmed) =====
			'editorUnnecessaryCode.opacity': '#00000070',

			// ===== Errors / warnings / info =====
			'editorError.foreground': red,
			'editorError.background': `${red}18`,
			'editorWarning.foreground': orange,
			'editorWarning.background': `${orange}18`,
			'editorInfo.foreground': blue,
			'editorInfo.background': `${blue}18`,
			'editorHint.foreground': gray700,

			// ===== Widget =====
			'editorWidget.background': gray0,
			'editorWidget.foreground': gray1000,
			'editorWidget.border': gray300,
			'editorWidget.resizeBorder': blue,

			// ===== Suggest widget =====
			'editorSuggestWidget.background': gray0,
			'editorSuggestWidget.foreground': gray1000,
			'editorSuggestWidget.border': gray300,
			'editorSuggestWidget.selectedBackground': gray400,
			'editorSuggestWidget.selectedForeground': gray1000,
			'editorSuggestWidget.selectedIconForeground': blue,
			'editorSuggestWidget.highlightForeground': blue,
			'editorSuggestWidget.focusHighlightForeground': blue,

			// ===== Inlay hints =====
			'editorInlayHint.foreground': gray600,
			'editorInlayHint.background': `${gray300}80`,
			'editorInlayHint.typeForeground': blue,
			'editorInlayHint.typeBackground': `${gray300}80`,
			'editorInlayHint.parameterForeground': purple,
			'editorInlayHint.parameterBackground': `${gray300}80`,

			// ===== Lightbulb =====
			'editorLightBulb.foreground': yellow,
			'editorLightBulbAutoFix.foreground': blue,

			// ===== Links =====
			'editorLink.activeForeground': blue,

			// ===== Sticky scroll =====
			'editorStickyScroll.background': gray0,
			'editorStickyScrollHover.background': gray200,
			'editorStickyScroll.border': gray300,

			// ===== Diff editor =====
			'diffEditor.insertedLineBackground': `${green}18`,
			'diffEditor.removedLineBackground': `${red}18`,
			'diffEditor.insertedTextBackground': `${green}30`,
			'diffEditor.removedTextBackground': `${red}30`,
			'diffEditorGutter.insertedLineBackground': `${green}40`,
			'diffEditorGutter.removedLineBackground': `${red}40`,
			'diffEditor.border': gray300,
			'diffEditor.diagonalFill': `${gray400}80`,

			// ===== List (suggest dropdown, quick pick) =====
			'list.activeSelectionBackground': gray400,
			'list.activeSelectionForeground': gray1000,
			'list.activeSelectionIconForeground': gray1000,
			'list.inactiveSelectionBackground': gray300,
			'list.inactiveSelectionForeground': gray1000,
			'list.hoverBackground': gray200,
			'list.hoverForeground': gray1000,
			'list.focusBackground': gray400,
			'list.focusForeground': gray1000,
			'list.focusOutline': blue,
			'list.highlightForeground': blue,
			'list.focusHighlightForeground': blue,
			'list.errorForeground': red,
			'list.warningForeground': orange,
			'list.filterMatchBackground': `${orange}40`,

			// ===== Quick input =====
			'quickInput.background': gray0,
			'quickInput.foreground': gray1000,
			'quickInputTitle.background': gray200,
			'quickInputList.focusBackground': gray400,
			'quickInputList.focusForeground': gray1000,
			'pickerGroup.foreground': blue,
			'pickerGroup.border': gray300,

			// ===== Scrollbar =====
			'scrollbar.shadow': `${gray0}00`,
			'scrollbarSlider.background': `${gray600}60`,
			'scrollbarSlider.hoverBackground': `${gray700}80`,
			'scrollbarSlider.activeBackground': gray700,

			// ===== Focus / global =====
			focusBorder: blue,
			'selection.background': gray600
		}
	};
};
