# vendor/forge — Vendored forge design system tarballs

Forge is consumed via packed tarballs, not the GitLab npm registry. This is an interim distribution model until forge moves to the Gradion GitLab group + group deploy token.

Forge is **atoms-only**: only `@shared/token` (tokens) and `@shared/ui` (atom components) are vendored. Molecules/organisms/templates live in workspace `gw-ui`.

## Files

| File | Source |
|------|--------|
| `shared-token-<version>.tgz` | `pnpm pack` of forge's `@shared/token` package |
| `shared-ui-<version>.tgz` | `pnpm pack` of forge's `@shared/ui` package |

## How to update forge (recommended)

Use the helper script. Assumes your forge checkout sits at `$HOME/Documents/Gradion/forge-design-system` — pass a different path as an argument if not. The script locates the two packages by their package.json `name` (`@shared/token` / `@shared/ui`), so it doesn't care what the package directories are called.

```bash
# From workspace repo root
scripts/update-forge.sh
# or with an explicit path
scripts/update-forge.sh /path/to/your/forge-design-system
```

What it does:
1. Finds the `@shared/token` + `@shared/ui` packages in the forge checkout
2. `pnpm pack` each
3. Clears old `vendor/forge/*.tgz`
4. Copies new tarballs in
5. Updates `file:` refs (`@shared/token` / `@shared/ui`) in `gw-design-system`, `gw-ui`, `gw-web`, `gw-admin-web` `package.json` files
6. Syncs the vendored `variables.mobile.css` copy into `gw-design-system`

Then:

```bash
rm -rf node_modules pnpm-lock.yaml
pnpm install
make fe-typecheck
pnpm --filter @gradion/ui test:unit
pnpm --filter @gradion/web test
pnpm --filter @gradion/admin-web test
make web-dev  # browser smoke
```

The three `test:unit` / `test` runs are the gate. They catch class-name
drift after a forge bump (e.g., a Tailwind utility consumer references that
forge no longer emits) and JSX-runtime config regressions. Run them every
bump.

Commit the `vendor/forge/` change + the four `package.json` bumps + the new `pnpm-lock.yaml` in one PR.

## Manual fallback (if the script breaks)

1. `pnpm pack` in each forge package (`@shared/token`, `@shared/ui`), copy the `.tgz` into this folder
2. Delete the previous versions from this folder
3. Update `file:../vendor/forge/...` refs in:
   - `gw-design-system/package.json` — `@shared/token`
   - `gw-ui/package.json` — `@shared/token` + `@shared/ui`
   - `gw-web/package.json` — `@shared/token` + `@shared/ui`
   - `gw-admin-web/package.json` — `@shared/token` + `@shared/ui`
4. Re-copy `src/theme/variables.mobile.css` from the `@shared/token` package into `gw-design-system/src/theme/variables.mobile.css`
5. `rm -rf node_modules pnpm-lock.yaml && pnpm install`

## Exit

This whole flow disappears when forge moves to the Gradion GitLab group + group deploy token. After that:
- `pnpm update @shared/token @shared/ui` in workspace
- `rm -rf vendor/forge` and drop the `file:` refs
- Add the scoped `.npmrc` registry block back
