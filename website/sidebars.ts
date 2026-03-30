import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'getting-started',
    'features',
    'agent-images',
    'architecture',
    'security',
    {
      type: 'category',
      label: 'Operations',
      items: [
        'operations/troubleshooting',
        'operations/releasing',
        'operations/upgrading',
      ],
    },
    'roadmap',
  ],
};

export default sidebars;
