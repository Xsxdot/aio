export default {
  // DNS Credentials
  'ssl.dnsCredentials.title': 'DNS Credentials',
  'ssl.dnsCredentials.create': 'Create DNS Credential',
  'ssl.dnsCredentials.edit': 'Edit DNS Credential',
  'ssl.dnsCredentials.search': 'Search',
  'ssl.dnsCredentials.reset': 'Reset',
  'ssl.dnsCredentials.refresh': 'Refresh',
  'ssl.dnsCredentials.search.placeholder': 'Enter credential name',

  'ssl.dnsCredentials.column.id': 'ID',
  'ssl.dnsCredentials.column.name': 'Credential Name',
  'ssl.dnsCredentials.column.provider': 'DNS Provider',
  'ssl.dnsCredentials.column.status': 'Status',
  'ssl.dnsCredentials.column.description': 'Description',
  'ssl.dnsCredentials.column.createdAt': 'Created At',
  'ssl.dnsCredentials.column.actions': 'Actions',

  'ssl.dnsCredentials.form.name': 'Credential Name',
  'ssl.dnsCredentials.form.name.placeholder': 'Enter credential name',
  'ssl.dnsCredentials.form.provider': 'DNS Provider',
  'ssl.dnsCredentials.form.provider.placeholder': 'Select DNS provider',
  'ssl.dnsCredentials.form.accessKey': 'Access Key',
  'ssl.dnsCredentials.form.accessKey.placeholder': 'Enter Access Key',
  'ssl.dnsCredentials.form.secretKey': 'Secret Key',
  'ssl.dnsCredentials.form.secretKey.placeholder': 'Enter Secret Key',
  'ssl.dnsCredentials.form.secretKey.edit.placeholder':
    'Leave blank to keep unchanged',
  'ssl.dnsCredentials.form.extraConfig': 'Extra Config',
  'ssl.dnsCredentials.form.extraConfig.placeholder':
    'Enter JSON format extra config',
  'ssl.dnsCredentials.form.description': 'Description',
  'ssl.dnsCredentials.form.description.placeholder': 'Enter description',
  'ssl.dnsCredentials.form.cancel': 'Cancel',
  'ssl.dnsCredentials.form.submit': 'Submit',

  'ssl.dnsCredentials.status.enabled': 'Enabled',
  'ssl.dnsCredentials.status.disabled': 'Disabled',

  'ssl.dnsCredentials.action.edit': 'Edit',
  'ssl.dnsCredentials.action.disable': 'Disable',
  'ssl.dnsCredentials.action.enable': 'Enable',
  'ssl.dnsCredentials.action.delete': 'Delete',

  'ssl.dnsCredentials.confirm.disable': 'Confirm Disable',
  'ssl.dnsCredentials.confirm.disable.content':
    'Are you sure to disable this DNS credential?',
  'ssl.dnsCredentials.confirm.enable': 'Confirm Enable',
  'ssl.dnsCredentials.confirm.enable.content':
    'Are you sure to enable this DNS credential?',
  'ssl.dnsCredentials.confirm.delete': 'Confirm Delete',
  'ssl.dnsCredentials.confirm.delete.content':
    'Are you sure to delete this DNS credential? This action cannot be undone.',

  'ssl.dnsCredentials.message.createSuccess': 'Created successfully',
  'ssl.dnsCredentials.message.updateSuccess': 'Updated successfully',
  'ssl.dnsCredentials.message.deleteSuccess': 'Deleted successfully',
  'ssl.dnsCredentials.message.statusUpdateSuccess':
    'Status updated successfully',

  'ssl.dnsCredentials.validation.name.required': 'Please enter credential name',
  'ssl.dnsCredentials.validation.provider.required':
    'Please select DNS provider',
  'ssl.dnsCredentials.validation.accessKey.required': 'Please enter Access Key',
  'ssl.dnsCredentials.validation.secretKey.required': 'Please enter Secret Key',

  // Certificates
  'ssl.certificates.title': 'Certificate Management',
  'ssl.certificates.issue': 'Issue Certificate',
  'ssl.certificates.search': 'Search',
  'ssl.certificates.reset': 'Reset',
  'ssl.certificates.refresh': 'Refresh',
  'ssl.certificates.search.placeholder': 'Enter certificate name or domain',

  'ssl.certificates.column.id': 'ID',
  'ssl.certificates.column.name': 'Certificate Name',
  'ssl.certificates.column.domain': 'Domain',
  'ssl.certificates.column.status': 'Status',
  'ssl.certificates.column.expiresAt': 'Expires At',
  'ssl.certificates.column.issuedAt': 'Issued At',
  'ssl.certificates.column.autoRenew': 'Auto Renew',
  'ssl.certificates.column.autoDeploy': 'Auto Deploy',
  'ssl.certificates.column.description': 'Description',
  'ssl.certificates.column.createdAt': 'Created At',
  'ssl.certificates.column.actions': 'Actions',

  'ssl.certificates.form.name': 'Certificate Name',
  'ssl.certificates.form.name.placeholder': 'Enter certificate name (optional)',
  'ssl.certificates.form.domain': 'Domain',
  'ssl.certificates.form.domain.placeholder': 'Enter domain (supports wildcard like *.a.com)',
  'ssl.certificates.form.email': 'Email',
  'ssl.certificates.form.email.placeholder': 'Enter email address',
  'ssl.certificates.form.dnsCredentialId': 'DNS Credential',
  'ssl.certificates.form.dnsCredentialId.placeholder': 'Select DNS credential',
  'ssl.certificates.form.renewBeforeDays': 'Renew Before Days',
  'ssl.certificates.form.renewBeforeDays.placeholder': 'Default: 30 days',
  'ssl.certificates.form.autoRenew': 'Auto Renew',
  'ssl.certificates.form.autoDeploy': 'Auto Deploy',
  'ssl.certificates.form.autoDeploy.tip': 'When enabled, certificates will be automatically matched and deployed to corresponding deploy targets by domain when issued or renewed',
  'ssl.certificates.form.useStaging': 'Use Staging Environment',
  'ssl.certificates.form.useStaging.tip':
    "Enable to use Let's Encrypt staging environment",
  'ssl.certificates.form.description': 'Description',
  'ssl.certificates.form.description.placeholder': 'Enter description',
  'ssl.certificates.form.cancel': 'Cancel',
  'ssl.certificates.form.submit': 'Submit',

  'ssl.certificates.status.pending': 'Pending',
  'ssl.certificates.status.issuing': 'Issuing',
  'ssl.certificates.status.active': 'Active',
  'ssl.certificates.status.renewing': 'Renewing',
  'ssl.certificates.status.expired': 'Expired',
  'ssl.certificates.status.failed': 'Failed',

  'ssl.certificates.action.renew': 'Renew',
  'ssl.certificates.action.deploy': 'Deploy',
  'ssl.certificates.action.history': 'Deploy History',
  'ssl.certificates.action.delete': 'Delete',

  'ssl.certificates.confirm.renew': 'Confirm Renew',
  'ssl.certificates.confirm.renew.content':
    'Are you sure to renew this certificate?',
  'ssl.certificates.confirm.delete': 'Confirm Delete',
  'ssl.certificates.confirm.delete.content':
    'Are you sure to delete this certificate? This action cannot be undone.',

  'ssl.certificates.deploy.title': 'Deploy Certificate',
  'ssl.certificates.deploy.selectTargets': 'Select Deploy Targets',
  'ssl.certificates.deploy.selectTargets.placeholder':
    'Select one or more deploy targets',
  'ssl.certificates.deploy.cancel': 'Cancel',
  'ssl.certificates.deploy.submit': 'Start Deploy',

  'ssl.certificates.history.title': 'Deploy History',
  'ssl.certificates.history.column.id': 'ID',
  'ssl.certificates.history.column.targetId': 'Target ID',
  'ssl.certificates.history.column.status': 'Status',
  'ssl.certificates.history.column.triggerType': 'Trigger Type',
  'ssl.certificates.history.column.startTime': 'Start Time',
  'ssl.certificates.history.column.endTime': 'End Time',
  'ssl.certificates.history.column.errorMessage': 'Error Message',

  'ssl.certificates.history.triggerType.manual': 'Manual',
  'ssl.certificates.history.triggerType.autoRenew': 'Auto Renew',
  'ssl.certificates.history.triggerType.autoIssue': 'Auto Issue',

  'ssl.certificates.history.deployStatus.pending': 'Pending',
  'ssl.certificates.history.deployStatus.deploying': 'Deploying',
  'ssl.certificates.history.deployStatus.success': 'Success',
  'ssl.certificates.history.deployStatus.failed': 'Failed',
  'ssl.certificates.history.deployStatus.partial': 'Partial',

  'ssl.certificates.message.issueSuccess': 'Certificate issuance submitted',
  'ssl.certificates.message.renewSuccess': 'Certificate renewal triggered',
  'ssl.certificates.message.deploySuccess': 'Certificate deployment triggered',
  'ssl.certificates.message.deleteSuccess': 'Deleted successfully',

  'ssl.certificates.validation.domain.required': 'Please enter domain',
  'ssl.certificates.validation.email.required': 'Please enter email address',
  'ssl.certificates.validation.email.format':
    'Please enter a valid email address',
  'ssl.certificates.validation.dnsCredentialId.required':
    'Please select DNS credential',
  'ssl.certificates.validation.deployTargets.required':
    'Please select at least one deploy target',

  // Deploy Targets
  'ssl.deployTargets.title': 'Deploy Targets',
  'ssl.deployTargets.create': 'Create Deploy Target',
  'ssl.deployTargets.edit': 'Edit Deploy Target',
  'ssl.deployTargets.search': 'Search',
  'ssl.deployTargets.reset': 'Reset',
  'ssl.deployTargets.refresh': 'Refresh',
  'ssl.deployTargets.search.placeholder': 'Enter target name',

  'ssl.deployTargets.column.id': 'ID',
  'ssl.deployTargets.column.name': 'Target Name',
  'ssl.deployTargets.column.domain': 'Domain',
  'ssl.deployTargets.column.type': 'Type',
  'ssl.deployTargets.column.status': 'Status',
  'ssl.deployTargets.column.description': 'Description',
  'ssl.deployTargets.column.createdAt': 'Created At',
  'ssl.deployTargets.column.actions': 'Actions',

  'ssl.deployTargets.form.name': 'Target Name',
  'ssl.deployTargets.form.name.placeholder': 'Enter target name',
  'ssl.deployTargets.form.domain': 'Bound Domain',
  'ssl.deployTargets.form.domain.placeholder': 'Enter domain (supports wildcard like *.a.com or b.a.com)',
  'ssl.deployTargets.form.type': 'Deploy Type',
  'ssl.deployTargets.form.type.placeholder': 'Select deploy type',
  'ssl.deployTargets.form.description': 'Description',
  'ssl.deployTargets.form.description.placeholder': 'Enter description',
  'ssl.deployTargets.form.cancel': 'Cancel',
  'ssl.deployTargets.form.submit': 'Submit',

  'ssl.deployTargets.type.local': 'Local File',
  'ssl.deployTargets.type.ssh': 'SSH Remote',
  'ssl.deployTargets.type.aliyunCas': 'Aliyun CAS',

  // Local config
  'ssl.deployTargets.local.basePath': 'Certificate Path',
  'ssl.deployTargets.local.basePath.placeholder': 'e.g.: /etc/nginx/ssl',
  'ssl.deployTargets.local.fullchainName': 'Certificate Filename',
  'ssl.deployTargets.local.fullchainName.placeholder': 'Auto-generated: domain.crt',
  'ssl.deployTargets.local.privkeyName': 'Private Key Filename',
  'ssl.deployTargets.local.privkeyName.placeholder': 'Auto-generated: domain.key',
  'ssl.deployTargets.local.fileMode': 'File Mode',
  'ssl.deployTargets.local.fileMode.placeholder': 'e.g.: 0600',
  'ssl.deployTargets.local.reloadCommand': 'Reload Command',
  'ssl.deployTargets.local.reloadCommand.placeholder': 'e.g.: nginx -s reload',

  // SSH config
  'ssl.deployTargets.ssh.server': 'Select Server',
  'ssl.deployTargets.ssh.server.placeholder': 'Select a server',
  'ssl.deployTargets.ssh.remotePath': 'Remote Certificate Path',
  'ssl.deployTargets.ssh.remotePath.placeholder': 'e.g.: /etc/nginx/ssl',
  'ssl.deployTargets.ssh.fullchainName': 'Certificate Filename',
  'ssl.deployTargets.ssh.fullchainName.placeholder': 'Default: fullchain.pem',
  'ssl.deployTargets.ssh.privkeyName': 'Private Key Filename',
  'ssl.deployTargets.ssh.privkeyName.placeholder': 'Default: privkey.pem',
  'ssl.deployTargets.ssh.fileMode': 'File Mode',
  'ssl.deployTargets.ssh.fileMode.placeholder': 'e.g.: 0600',
  'ssl.deployTargets.ssh.reloadCommand': 'Reload Command',
  'ssl.deployTargets.ssh.reloadCommand.placeholder': 'e.g.: nginx -s reload',

  // Aliyun CAS config
  'ssl.deployTargets.aliyun.dnsCredential': 'DNS Credential',
  'ssl.deployTargets.aliyun.dnsCredential.placeholder': 'Select DNS credential (Aliyun)',
  'ssl.deployTargets.aliyun.region': 'Region',
  'ssl.deployTargets.aliyun.region.placeholder': 'Default: cn-hangzhou',
  'ssl.deployTargets.aliyun.autoDeploy': 'Auto Deploy to Cloud Products',

  'ssl.deployTargets.status.enabled': 'Enabled',
  'ssl.deployTargets.status.disabled': 'Disabled',

  'ssl.deployTargets.action.edit': 'Edit',
  'ssl.deployTargets.action.disable': 'Disable',
  'ssl.deployTargets.action.enable': 'Enable',
  'ssl.deployTargets.action.delete': 'Delete',

  'ssl.deployTargets.confirm.disable': 'Confirm Disable',
  'ssl.deployTargets.confirm.disable.content':
    'Are you sure to disable this deploy target?',
  'ssl.deployTargets.confirm.enable': 'Confirm Enable',
  'ssl.deployTargets.confirm.enable.content':
    'Are you sure to enable this deploy target?',
  'ssl.deployTargets.confirm.delete': 'Confirm Delete',
  'ssl.deployTargets.confirm.delete.content':
    'Are you sure to delete this deploy target? This action cannot be undone.',

  'ssl.deployTargets.message.createSuccess': 'Created successfully',
  'ssl.deployTargets.message.updateSuccess': 'Updated successfully',
  'ssl.deployTargets.message.deleteSuccess': 'Deleted successfully',
  'ssl.deployTargets.message.statusUpdateSuccess':
    'Status updated successfully',

  'ssl.deployTargets.validation.name.required': 'Please enter target name',
  'ssl.deployTargets.validation.domain.required': 'Please enter domain',
  'ssl.deployTargets.validation.type.required': 'Please select deploy type',
  'ssl.deployTargets.validation.basePath.required':
    'Please enter certificate path',
  'ssl.deployTargets.validation.serverId.required': 'Please select a server',
  'ssl.deployTargets.validation.remotePath.required':
    'Please enter remote certificate path',
  'ssl.deployTargets.validation.dnsCredentialId.required':
    'Please select DNS credential',
};
