// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import mermaid from 'astro-mermaid';

// https://astro.build/config
export default defineConfig({
	site: 'https://docs.askrelay.dev',
	integrations: [
		// astro-mermaid must precede starlight: it rewrites ```mermaid fences
		// into client-rendered, theme-aware diagrams.
		mermaid({ autoTheme: true }),
		starlight({
			title: 'askrelay',
			description: 'Your AI can ask my AI — async, approval-gated messaging between people’s AI sessions.',
			logo: { src: './src/assets/askrelay-mark.png', alt: 'askrelay' },
			favicon: '/favicons/favicon.ico',
			customCss: ['./src/styles/brand.css'],
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/Mediacom99/askrelay' }],
			editLink: { baseUrl: 'https://github.com/Mediacom99/askrelay/edit/main/website/' },
			head: [
				{ tag: 'meta', attrs: { name: 'theme-color', content: '#0A121D' } },
				{ tag: 'link', attrs: { rel: 'manifest', href: '/site.webmanifest' } },
				{ tag: 'link', attrs: { rel: 'apple-touch-icon', href: '/favicons/apple-touch-icon.png' } },
			],
			sidebar: [
				{ label: 'Overview', link: '/' },
				{ label: 'Quickstart', slug: 'quickstart' },
				{ label: 'Use cases', slug: 'use-cases' },
				{
					label: 'Guides',
					items: [
						{ label: 'Connect a client', slug: 'connect-a-client' },
						{ label: 'Self-host & operate', slug: 'self-host' },
					],
				},
				{ label: 'Concepts', slug: 'concepts' },
				{ label: 'Security & threat model', slug: 'security' },
				{ label: 'Reference', slug: 'reference' },
				{ label: 'Contributing', slug: 'contributing' },
			],
		}),
	],
});
