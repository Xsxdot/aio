import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const CREDENTIAL: AppRouteRecordRaw = {
  path: '/credential',
  name: 'credential',
  component: DEFAULT_LAYOUT,
  redirect: '/credential/client-credentials',
  meta: {
    locale: 'menu.credential',
    requiresAuth: true,
    icon: 'icon-safe',
    order: 3,
    hideChildrenInMenu: true,
  },
  children: [
    {
      path: 'client-credentials',
      name: 'ClientCredentials',
      component: () => import('@/views/client-credentials/index.vue'),
      meta: {
        locale: 'menu.credential.clientCredentials',
        requiresAuth: true,
        roles: ['*'],
        hideInMenu: true,
        activeMenu: 'credential',
      },
    },
  ],
};

export default CREDENTIAL;





