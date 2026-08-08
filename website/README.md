# askrelay docs site

The public documentation for askrelay — [Astro](https://astro.build) +
[Starlight](https://starlight.astro.build), branded, with Mermaid diagrams.
Content lives in `src/content/docs/` as Markdown/MDX.

## Preview locally

```sh
cd website
npm install
npm run dev        # http://localhost:4321
```

Build the production site and preview the built output:

```sh
npm run build      # -> website/dist
npm run preview
```

Node is pinned to 22 (`.nvmrc`) to match the Cloudflare build image; a newer
local Node is fine.

## Deploy — Cloudflare Pages (recommended: Git integration)

Nothing here publishes automatically. To go live, connect the repo once in the
Cloudflare dashboard:

1. **Workers & Pages → Create → Pages → Connect to Git** → pick `Mediacom99/askrelay`.
2. Build settings:
   - **Root directory:** `website`
   - **Build command:** `npm run build`
   - **Build output directory:** `dist`
   - **Environment variable:** `NODE_VERSION = 22` (or rely on `.nvmrc`)
   - **Production branch:** `main`
3. Deploy. Every pull request then gets an automatic **preview URL**
   (`<hash>.askrelay-docs.pages.dev`) posted as a check; only `main` publishes to
   production.
4. **Custom domain:** Pages → Custom domains → add `docs.askrelay.dev` (creates
   the CNAME on the zone).

CLI / CI alternative (`wrangler.toml` is present): `npx wrangler pages deploy dist`.
