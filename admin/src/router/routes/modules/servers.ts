import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const SERVERS: AppRouteRecordRaw = {
  path: '/servers',
  name: 'servers',
  component: DEFAULT_LAYOUT,
  redirect: '/servers/manage',
  meta: {
    locale: 'menu.servers',
    requiresAuth: true,
    icon: 'icon-desktop',
    order: 5,
    hideChildrenInMenu: true,
  },
  children: [
    {
      path: 'manage',
      name: 'ServersManage',
      component: () => import('@/views/servers/index.vue'),
      meta: {
        locale: 'menu.servers.manage',
        requiresAuth: true,
        roles: ['admin'],
        hideInMenu: true,
        activeMenu: 'servers',
      },
    },
  ],
};

export default SERVERS;
