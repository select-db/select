# Theme File

The `.theme` file controls the visual appearance of SELECT. It lives at the root of your workspace and uses standard CSS custom properties.

## Format

The file has three sections: **shared** variables (applied in both modes), **light** mode colors, and **dark** mode colors.

```css
:root {
  --fs-sm: 0.75rem;
  --fw-normal: 400;
  --bw: 0.5px;
  --br-md: 0.25rem;
}

:root[data-theme="light"] {
  --hue: 40;
  --gray-0: hsl(var(--hue), 12%, 98%);
  --gray-900: hsl(var(--hue), 25%, 22%);
  --red: rgb(158, 45, 45);
}

:root[data-theme="dark"] {
  --hue: 210;
  --gray-0: hsl(var(--hue), 14%, 10%);
  --gray-900: hsl(var(--hue), 10%, 88%);
  --red: rgb(230, 115, 115);
}
```

## What you can customize

You only need to include the variables you want to override. SELECT merges your file with the built-in defaults.

**Available variable groups:**

- **Font sizes**: `--fs-xs`, `--fs-sm`, `--fs-md`, `--fs-lg`, `--fs-xl`, `--fs-xxl`
- **Font weights**: `--fw-light`, `--fw-normal`, `--fw-bold`
- **Border**: `--bw` (width), `--br-xs`, `--br-sm`, `--br-md` (radii)
- **Gray scale**: `--gray-0` through `--gray-1050` (background, text, borders)
- **Accent colors**: `--red`, `--blue`, `--green`, `--yellow`, `--orange`, `--purple` (and dark variants)
- **Glow effects**: `--red-glow`, `--green-glow`, `--blue-glow`, `--white-glow`
- **Hue**: `--hue` controls the base hue for the gray scale (light defaults to `40`, dark to `210`)

## Applying changes

After editing the `.theme` file, click **Apply** in the theme editor to update the UI. To restore the built-in defaults, click **Reset**.
