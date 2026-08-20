import localeMessageBox from '@/components/message-box/locale/en-US';
import localeLogin from '@/views/login/locale/en-US';

import localeWorkplace from '@/views/dashboard/workplace/locale/en-US';
import localeConfig from '@/views/config/locale/en-US';
import localeClientCredentials from '@/views/client-credentials/locale/en-US';
import localeAdmins from '@/views/admins/locale/en-US';
import localeSsl from '@/views/ssl/locale/en-US';
import localeRegistry from '@/views/registry/locale/en-US';
import localeServers from '@/views/servers/locale/en-US';
import localeShortUrlDomains from '@/views/shorturl/domains/locale/en-US';
import localeShortUrlLinks from '@/views/shorturl/links/locale/en-US';
import localeExecutorJobs from '@/views/executor/jobs/locale/en-US';

import localeSettings from './en-US/settings';

export default {
  'menu.dashboard': 'Dashboard',
  'menu.server.dashboard': 'Dashboard-Server',
  'menu.server.workplace': 'Workplace-Server',
  'menu.server.monitor': 'Monitor-Server',
  'menu.list': 'List',
  'menu.result': 'Result',
  'menu.exception': 'Exception',
  'menu.form': 'Form',
  'menu.profile': 'Profile',
  'menu.visualization': 'Data Visualization',
  'menu.user': 'User Center',
  'menu.config': 'Config Center',
  'menu.config.center': 'Config Management',
  'menu.credential': 'Credential Management',
  'menu.credential.clientCredentials': 'Client Credentials',
  'menu.ssl': 'SSL Management',
  'menu.ssl.dnsCredentials': 'DNS Credentials',
  'menu.ssl.certificates': 'Certificates',
  'menu.ssl.deployTargets': 'Deploy Targets',
  'menu.registry': 'Service Registry',
  'menu.registry.services': 'Service Management',
  'menu.shorturl': 'Short URL',
  'menu.shorturl.domains': 'Domain Management',
  'menu.shorturl.links': 'Link Management',
  'menu.system': 'System Management',
  'menu.system.admins': 'Admin Management',
  'navbar.docs': 'Docs',
  'navbar.action.locale': 'Switch to English',
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
