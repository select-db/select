export const getCSSVariable = (name: string) => {
	return getComputedStyle(document.documentElement).getPropertyValue(name);
};
