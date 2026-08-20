export default {
  'clientCredentials.list.title': '客户端凭证列表',
  'clientCredentials.search.placeholder': '搜索客户端Key或名称',
  'clientCredentials.search': '搜索',
  'clientCredentials.reset': '重置',
  'clientCredentials.create': '新增客户端凭证',
  'clientCredentials.refresh': '刷新',
  'clientCredentials.empty': '暂无客户端凭证',

  // 表格列
  'clientCredentials.column.id': 'ID',
  'clientCredentials.column.name': '名称',
  'clientCredentials.column.clientKey': '客户端Key',
  'clientCredentials.column.status': '状态',
  'clientCredentials.column.description': '描述',
  'clientCredentials.column.ipWhitelist': 'IP白名单',
  'clientCredentials.column.expiresAt': '过期时间',
  'clientCredentials.column.createdAt': '创建时间',
  'clientCredentials.column.updatedAt': '更新时间',
  'clientCredentials.column.actions': '操作',

  // 状态
  'clientCredentials.status.enabled': '启用',
  'clientCredentials.status.disabled': '禁用',
  'clientCredentials.status.neverExpires': '永不过期',

  // 表单
  'clientCredentials.form.create.title': '新增客户端凭证',
  'clientCredentials.form.edit.title': '编辑客户端凭证',
  'clientCredentials.form.name': '名称',
  'clientCredentials.form.name.placeholder': '请输入客户端名称（至少3个字符）',
  'clientCredentials.form.description': '描述',
  'clientCredentials.form.description.placeholder': '请输入客户端描述（选填）',
  'clientCredentials.form.ipWhitelist': 'IP白名单',
  'clientCredentials.form.ipWhitelist.placeholder':
    '按回车添加IP地址（留空表示不限制）',
  'clientCredentials.form.expiresAt': '过期时间',
  'clientCredentials.form.expiresAt.placeholder':
    '选择过期时间（不选表示永不过期）',
  'clientCredentials.form.submit': '提交',
  'clientCredentials.form.cancel': '取消',

  // 验证提示
  'clientCredentials.validation.name.required': '请输入客户端名称',
  'clientCredentials.validation.name.minLength': '客户端名称至少3个字符',
  'clientCredentials.validation.name.maxLength': '客户端名称最多200个字符',

  // 操作按钮
  'clientCredentials.action.edit': '编辑',
  'clientCredentials.action.disable': '禁用',
  'clientCredentials.action.enable': '启用',
  'clientCredentials.action.rotateSecret': '轮换密钥',
  'clientCredentials.action.delete': '删除',
  'clientCredentials.action.viewSecret': '查看密钥',

  // 确认对话框
  'clientCredentials.confirm.delete.title': '确认删除',
  'clientCredentials.confirm.delete.content':
    '确定要删除该客户端凭证吗？删除后无法恢复。',
  'clientCredentials.confirm.disable.title': '确认禁用',
  'clientCredentials.confirm.disable.content':
    '确定要禁用该客户端凭证吗？禁用后将从列表中消失（仅显示启用且未过期的客户端）。',
  'clientCredentials.confirm.rotateSecret.title': '确认轮换密钥',
  'clientCredentials.confirm.rotateSecret.content':
    '确定要重新生成密钥吗？旧密钥将立即失效。',

  // 密钥展示弹窗
  'clientCredentials.secret.title': '客户端密钥信息',
  'clientCredentials.secret.warning': '客户端密钥只会展示一次，请妥善保管！',
  'clientCredentials.secret.clientKey': '客户端Key',
  'clientCredentials.secret.clientSecret': '客户端Secret',
  'clientCredentials.secret.name': '名称',
  'clientCredentials.secret.description': '描述',
  'clientCredentials.secret.copy': '复制',
  'clientCredentials.secret.copied': '已复制',
  'clientCredentials.secret.close': '关闭',

  // 提示消息
  'clientCredentials.message.create.success': '客户端凭证创建成功',
  'clientCredentials.message.create.failed': '创建客户端凭证失败',
  'clientCredentials.message.update.success': '客户端凭证更新成功',
  'clientCredentials.message.update.failed': '更新客户端凭证失败',
  'clientCredentials.message.delete.success': '客户端凭证删除成功',
  'clientCredentials.message.delete.failed': '删除客户端凭证失败',
  'clientCredentials.message.disable.success':
    '客户端凭证已禁用（刷新后将从列表消失）',
  'clientCredentials.message.disable.failed': '禁用客户端凭证失败',
  'clientCredentials.message.enable.success': '客户端凭证已启用',
  'clientCredentials.message.enable.failed': '启用客户端凭证失败',
  'clientCredentials.message.rotateSecret.success': '密钥轮换成功',
  'clientCredentials.message.rotateSecret.failed': '密钥轮换失败',
  'clientCredentials.message.load.failed': '加载客户端凭证列表失败',
  'clientCredentials.message.secretOnlyOnce':
    '密钥只在创建时展示一次，无法再次查看',
};





