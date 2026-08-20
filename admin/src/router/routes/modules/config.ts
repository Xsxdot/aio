import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const CONFIG: AppRouteRecordRaw = {
  path: '/config',
  name: 'config',
  component: DEFAULT_LAYOUT,
  redirect: '/config/center',
  meta: {
    locale: 'menu.config',
    requiresAuth: true,
    icon: 'icon-settings',
    order: 2,
    hideChildrenInMenu: true,
  },
  children: [
    {
      path: 'center',
      name: 'ConfigCenter',
      component: () => import('@/views/config/index.vue'),
      meta: {
        locale: 'menu.config.center',
        requiresAuth: true,
        roles: ['*'],
        hideInMenu: true,
        activeMenu: 'config',
      },
    },
  ],
};

export default CONFIG;





