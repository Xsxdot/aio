import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const SSL: AppRouteRecordRaw = {
  path: '/ssl',
  name: 'ssl',
  component: DEFAULT_LAYOUT,
  meta: {
    locale: 'menu.ssl',
    requiresAuth: true,
    icon: 'icon-lock',
    order: 5,
  },
  children: [
    {
      path: 'dns-credentials',
      name: 'DnsCredentials',
      component: () => import('@/views/ssl/dns-credentials/index.vue'),
      meta: {
        locale: 'menu.ssl.dnsCredentials',
        requiresAuth: true,
        roles: ['*'],
      },
    },
    {
      path: 'certificates',
      name: 'Certificates',
      component: () => import('@/views/ssl/certificates/index.vue'),
      meta: {
        locale: 'menu.ssl.certificates',
        requiresAuth: true,
        roles: ['*'],
      },
    },
    {
      path: 'deploy-targets',
      name: 'DeployTargets',
      component: () => import('@/views/ssl/deploy-targets/index.vue'),
      meta: {
        locale: 'menu.ssl.deployTargets',
        requiresAuth: true,
        roles: ['*'],
      },
    },
  ],
};

export default SSL;
