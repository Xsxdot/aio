export default {
  'clientCredentials.list.title': 'Client Credentials List',
  'clientCredentials.search.placeholder': 'Search by Client Key or Name',
  'clientCredentials.search': 'Search',
  'clientCredentials.reset': 'Reset',
  'clientCredentials.create': 'Create Client Credential',
  'clientCredentials.refresh': 'Refresh',
  'clientCredentials.empty': 'No client credentials',

  // Table columns
  'clientCredentials.column.id': 'ID',
  'clientCredentials.column.name': 'Name',
  'clientCredentials.column.clientKey': 'Client Key',
  'clientCredentials.column.status': 'Status',
  'clientCredentials.column.description': 'Description',
  'clientCredentials.column.ipWhitelist': 'IP Whitelist',
  'clientCredentials.column.expiresAt': 'Expires At',
  'clientCredentials.column.createdAt': 'Created At',
  'clientCredentials.column.updatedAt': 'Updated At',
  'clientCredentials.column.actions': 'Actions',

  // Status
  'clientCredentials.status.enabled': 'Enabled',
  'clientCredentials.status.disabled': 'Disabled',
  'clientCredentials.status.neverExpires': 'Never expires',

  // Form
  'clientCredentials.form.create.title': 'Create Client Credential',
  'clientCredentials.form.edit.title': 'Edit Client Credential',
  'clientCredentials.form.name': 'Name',
  'clientCredentials.form.name.placeholder':
    'Enter client name (at least 3 characters)',
  'clientCredentials.form.description': 'Description',
  'clientCredentials.form.description.placeholder':
    'Enter description (optional)',
  'clientCredentials.form.ipWhitelist': 'IP Whitelist',
  'clientCredentials.form.ipWhitelist.placeholder':
    'Press enter to add IP address (leave empty for no restriction)',
  'clientCredentials.form.expiresAt': 'Expires At',
  'clientCredentials.form.expiresAt.placeholder':
    'Select expiration time (leave empty for never expires)',
  'clientCredentials.form.submit': 'Submit',
  'clientCredentials.form.cancel': 'Cancel',

  // Validation
  'clientCredentials.validation.name.required': 'Please enter client name',
  'clientCredentials.validation.name.minLength':
    'Client name must be at least 3 characters',
  'clientCredentials.validation.name.maxLength':
    'Client name must not exceed 200 characters',

  // Action buttons
  'clientCredentials.action.edit': 'Edit',
  'clientCredentials.action.disable': 'Disable',
  'clientCredentials.action.enable': 'Enable',
  'clientCredentials.action.rotateSecret': 'Rotate Secret',
  'clientCredentials.action.delete': 'Delete',
  'clientCredentials.action.viewSecret': 'View Secret',

  // Confirm dialogs
  'clientCredentials.confirm.delete.title': 'Confirm Delete',
  'clientCredentials.confirm.delete.content':
    'Are you sure you want to delete this client credential? This action cannot be undone.',
  'clientCredentials.confirm.disable.title': 'Confirm Disable',
  'clientCredentials.confirm.disable.content':
    'Are you sure you want to disable this client credential? It will disappear from the list after refresh (only enabled and non-expired clients are shown).',
  'clientCredentials.confirm.rotateSecret.title': 'Confirm Rotate Secret',
  'clientCredentials.confirm.rotateSecret.content':
    'Are you sure you want to rotate the secret? The old secret will be invalidated immediately.',

  // Secret display modal
  'clientCredentials.secret.title': 'Client Credential Secret',
  'clientCredentials.secret.warning':
    'The client secret will only be displayed once. Please save it securely!',
  'clientCredentials.secret.clientKey': 'Client Key',
  'clientCredentials.secret.clientSecret': 'Client Secret',
  'clientCredentials.secret.name': 'Name',
  'clientCredentials.secret.description': 'Description',
  'clientCredentials.secret.copy': 'Copy',
  'clientCredentials.secret.copied': 'Copied',
  'clientCredentials.secret.close': 'Close',

  // Messages
  'clientCredentials.message.create.success':
    'Client credential created successfully',
  'clientCredentials.message.create.failed':
    'Failed to create client credential',
  'clientCredentials.message.update.success':
    'Client credential updated successfully',
  'clientCredentials.message.update.failed':
    'Failed to update client credential',
  'clientCredentials.message.delete.success':
    'Client credential deleted successfully',
  'clientCredentials.message.delete.failed':
    'Failed to delete client credential',
  'clientCredentials.message.disable.success':
    'Client credential disabled (will disappear from list after refresh)',
  'clientCredentials.message.disable.failed':
    'Failed to disable client credential',
  'clientCredentials.message.enable.success':
    'Client credential enabled successfully',
  'clientCredentials.message.enable.failed':
    'Failed to enable client credential',
  'clientCredentials.message.rotateSecret.success':
    'Secret rotated successfully',
  'clientCredentials.message.rotateSecret.failed': 'Failed to rotate secret',
  'clientCredentials.message.load.failed':
    'Failed to load client credentials list',
  'clientCredentials.message.secretOnlyOnce':
    'Secret is only displayed once during creation and cannot be viewed again',
};





