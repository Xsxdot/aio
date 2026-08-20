import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const REGISTRY: AppRouteRecordRaw = {
  path: '/registry',
  name: 'registry',
  component: DEFAULT_LAYOUT,
  redirect: '/registry/services',
  meta: {
    locale: 'menu.registry',
    requiresAuth: true,
    icon: 'icon-apps',
    order: 6,
    hideChildrenInMenu: true,
  },
  children: [
    {
      path: 'services',
      name: 'RegistryServices',
      component: () => import('@/views/registry/services/index.vue'),
      meta: {
        locale: 'menu.registry.services',
        requiresAuth: true,
        roles: ['admin'],
        hideInMenu: true,
        activeMenu: 'registry',
      },
    },
  ],
};

export default REGISTRY;





