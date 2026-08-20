import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const EXECUTOR: AppRouteRecordRaw = {
  path: '/executor',
  name: 'executor',
  component: DEFAULT_LAYOUT,
  redirect: '/executor/jobs',
  meta: {
    locale: 'menu.executor',
    requiresAuth: true,
    icon: 'icon-clock-circle',
    order: 7,
    hideChildrenInMenu: true,
  },
  children: [
    {
      path: 'jobs',
      name: 'ExecutorJobs',
      component: () => import('@/views/executor/jobs/index.vue'),
      meta: {
        locale: 'menu.executor.jobs',
        requiresAuth: true,
        roles: ['admin'],
        hideInMenu: true,
        activeMenu: 'executor',
      },
    },
  ],
};

export default EXECUTOR;
