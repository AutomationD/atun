import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Atun",
  description: "Seamless, IAM-native access to private RDS, Elasticache, DynamoDB, and more. No VPNs, no SSH agents, no friction.",
  srcDir: 'docs',
  outDir: '.vitepress/dist',
  
  // Ensure Alpine repository files are copied as-is
  assetsDir: 'repo',
  vite: {
    build: {
      rollupOptions: {
        input: {
          main: 'index.html',
          apk: 'repo/apk/index.html'
        }
      }
    }
  },

  appearance: false,
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config

    search: {
      provider: 'local'
    },
    nav: [
      // { text: 'Home', link: '/' },
      // { text: 'Examples', link: '/markdown-examples' }
    ],

    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Introduction', link: '/guide/' },
          { text: 'Quick Start', link: '/guide/quickstart' },
        ]
      },
      {
        text: 'Features',
        items: [
          { text: 'EC2 Router', link: '/guide/ec2-router' },
          { text: 'Tag Schema', link: '/guide/tag-schema' }
        ]
      },
      {
        text: 'Reference',
        items: [
          { text: 'CLI Commands', link: '/reference/cli-commands' },
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/automationd/atun' }
    ],

    footer: {
      message: 'Released under Apache 2.0 License.',
      copyright: 'Copyright 2025 Dmitry Kireev'
    }
  }
})
