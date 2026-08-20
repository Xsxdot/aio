import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const SYSTEM: AppRouteRecordRaw = {
  path: '/system',
  name: 'system',
  component: DEFAULT_LAYOUT,
  redirect: '/system/admins',
  meta: {
    locale: 'menu.system',
    requiresAuth: true,
    icon: 'icon-settings',
    order: 10,
    hideChildrenInMenu: true,
  },
  children: [
    {
      path: 'admins',
      name: 'Admins',
      component: () => import('@/views/admins/index.vue'),
      meta: {
        locale: 'menu.system.admins',
        requiresAuth: true,
        roles: ['admin'],
        hideInMenu: true,
        activeMenu: 'system',
      },
    },
  ],
};

export default SYSTEM;





