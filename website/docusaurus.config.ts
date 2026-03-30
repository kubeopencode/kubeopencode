import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'KubeOpenCode',
  tagline: 'Kubernetes-native Agent Platform for Teams and Enterprise',
  favicon: 'img/logo-s.png',

  future: {
    v4: true,
  },

  url: 'https://kubeopencode.github.io',
  baseUrl: '/kubeopencode/',

  organizationName: 'kubeopencode',
  projectName: 'kubeopencode',

  onBrokenLinks: 'warn',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl:
            'https://github.com/kubeopencode/kubeopencode/tree/main/website/',
        },
        blog: {
          showReadingTime: true,
          feedOptions: {
            type: ['rss', 'atom'],
            xslt: true,
          },
          editUrl:
            'https://github.com/kubeopencode/kubeopencode/tree/main/website/',
          onInlineTags: 'warn',
          onInlineAuthors: 'warn',
          onUntruncatedBlogPosts: 'warn',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/logo.png',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'KubeOpenCode',
      logo: {
        alt: 'KubeOpenCode Logo',
        src: 'img/logo-s.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {to: '/blog', label: 'Blog', position: 'left'},
        {
          href: 'https://github.com/kubeopencode/kubeopencode',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {
              label: 'Getting Started',
              to: '/docs/getting-started',
            },
            {
              label: 'Architecture',
              to: '/docs/architecture',
            },
            {
              label: 'Features',
              to: '/docs/features',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'Slack',
              href: 'https://join.slack.com/t/kubeopencode/shared_invite/zt-3o9qibz2b-PjJP4m2cHMcNT3cVg2TDhA',
            },
            {
              label: 'GitHub Discussions',
              href: 'https://github.com/kubeopencode/kubeopencode/discussions',
            },
            {
              label: 'GitHub Issues',
              href: 'https://github.com/kubeopencode/kubeopencode/issues',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'Blog',
              to: '/blog',
            },
            {
              label: 'GitHub',
              href: 'https://github.com/kubeopencode/kubeopencode',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} KubeOpenCode Contributors. Apache License 2.0.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'yaml', 'json', 'go', 'docker'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
