import localeMessageBox from '@/components/message-box/locale/zh-CN';
import localeLogin from '@/views/login/locale/zh-CN';

import localeWorkplace from '@/views/dashboard/workplace/locale/zh-CN';
import localeConfig from '@/views/config/locale/zh-CN';
import localeClientCredentials from '@/views/client-credentials/locale/zh-CN';
import localeAdmins from '@/views/admins/locale/zh-CN';
import localeSsl from '@/views/ssl/locale/zh-CN';
import localeRegistry from '@/views/registry/locale/zh-CN';
import localeServers from '@/views/servers/locale/zh-CN';
import localeShortUrlDomains from '@/views/shorturl/domains/locale/zh-CN';
import localeShortUrlLinks from '@/views/shorturl/links/locale/zh-CN';
import localeExecutorJobs from '@/views/executor/jobs/locale/zh-CN';

import localeSettings from './zh-CN/settings';

export default {
  'menu.dashboard': '仪表盘',
  'menu.server.dashboard': '仪表盘-服务端',
  'menu.server.workplace': '工作台-服务端',
  'menu.server.monitor': '实时监控-服务端',
  'menu.list': '列表页',
  'menu.result': '结果页',
  'menu.exception': '异常页',
  'menu.form': '表单页',
  'menu.profile': '详情页',
  'menu.visualization': '数据可视化',
  'menu.user': '个人中心',
  'menu.config': '配置中心',
  'menu.config.center': '配置管理',
  'menu.credential': '凭证管理',
  'menu.credential.clientCredentials': '客户端凭证',
  'menu.ssl': 'SSL 管理',
  'menu.ssl.dnsCredentials': 'DNS 凭证',
  'menu.ssl.certificates': '证书管理',
  'menu.ssl.deployTargets': '部署目标',
  'menu.registry': '服务注册中心',
  'menu.registry.services': '服务管理',
  'menu.shorturl': '短链接',
  'menu.shorturl.domains': '域名管理',
  'menu.shorturl.links': '链接管理',
  'menu.system': '系统管理',
  'menu.system.admins': '管理员管理',
  'navbar.docs': '文档中心',
  'navbar.action.locale': '切换为中文',
  ...localeSettings,
  ...localeMessageBox,
  ...localeLogin,
  ...localeWorkplace,
  ...localeConfig,
  ...localeClientCredentials,
  ...localeAdmins,
  ...localeSsl,
  ...localeRegistry,
  ...localeServers,
  ...localeShortUrlDomains,
  ...localeShortUrlLinks,
  ...localeExecutorJobs,
};
