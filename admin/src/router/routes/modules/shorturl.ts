import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const SHORTURL: AppRouteRecordRaw = {
  path: '/shorturl',
  name: 'shorturl',
  component: DEFAULT_LAYOUT,
  meta: {
    locale: 'menu.shorturl',
    requiresAuth: true,
    icon: 'icon-link',
    order: 6,
  },
  children: [
    {
      path: 'domains',
      name: 'ShortUrlDomains',
      component: () => import('@/views/shorturl/domains/index.vue'),
      meta: {
        locale: 'menu.shorturl.domains',
        requiresAuth: true,
        roles: ['*'],
      },
    },
    {
      path: 'links',
      name: 'ShortUrlLinks',
      component: () => import('@/views/shorturl/links/index.vue'),
      meta: {
        locale: 'menu.shorturl.links',
        requiresAuth: true,
        roles: ['*'],
      },
    },
  ],
};

export default SHORTURL;
