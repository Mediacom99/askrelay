# askrelay brand assets

## GitHub

- `github/github-avatar-500x500.png` — repository/organization/avatar icon
- `github/github-social-preview-1280x640.jpg` — upload in repository Settings → General → Social preview
- `github/github-readme-icon-256x256.png` — compact transparent README icon

Recommended README header:

```html
<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/askrelay-mark-dark.png" />
    <img src="docs/assets/askrelay-mark.png" alt="askrelay" width="120" />
  </picture>
  <h1>askrelay</h1>
  <p><strong>Your AI can ask my AI.</strong></p>
</div>
```

The `<picture>` swaps the navy mark (light theme) for the white mark
(`askrelay-mark-dark.png`) on GitHub's dark theme — the transparent pigeon reads
on both. The mark is the messenger pigeon whose wing doubles as a forward arrow.

## Website `<head>`

```html
<link rel="icon" href="/favicons/favicon.ico" sizes="any">
<link rel="icon" type="image/png" sizes="32x32" href="/favicons/favicon-32x32.png">
<link rel="icon" type="image/png" sizes="16x16" href="/favicons/favicon-16x16.png">
<link rel="apple-touch-icon" sizes="180x180" href="/favicons/apple-touch-icon.png">
<link rel="manifest" href="/site.webmanifest">

<meta property="og:image" content="https://YOUR-DOMAIN/og-image-1200x630.jpg">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:image" content="https://YOUR-DOMAIN/x-card-1200x600.jpg">
<meta name="theme-color" content="#0A121D">
```

## Included sizes

- Favicons: 16, 32, 48 px and multi-resolution `.ico`
- Apple touch icon: 180 px
- Android/PWA icons: 192 and 512 px
- GitHub avatar: 500 px
- GitHub social preview: 1280 × 640
- Open Graph: 1200 × 630
- X/Twitter card: 1200 × 600
- Transparent README marks: 64, 128, 256, 512, 1024 px
- Full lockups: 256, 512, 1024 px
