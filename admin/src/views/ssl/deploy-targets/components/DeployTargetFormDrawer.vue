<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="isEdit ? $t('ssl.deployTargets.edit') : $t('ssl.deployTargets.create')"
    :width="600"
    @cancel="handleCancel"
    @before-ok="handleSubmit"
  >
    <a-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      layout="vertical"
    >
      <a-form-item field="name" :label="$t('ssl.deployTargets.form.name')">
        <a-input
          v-model="formData.name"
          :placeholder="$t('ssl.deployTargets.form.name.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="domain" :label="$t('ssl.deployTargets.form.domain')">
        <a-input
          v-model="formData.domain"
          :placeholder="$t('ssl.deployTargets.form.domain.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="type" :label="$t('ssl.deployTargets.form.type')">
        <a-select
          v-model="formData.type"
          :placeholder="$t('ssl.deployTargets.form.type.placeholder')"
          :disabled="isEdit"
          @change="handleTypeChange"
        >
          <a-option value="local">{{ $t('ssl.deployTargets.type.local') }}</a-option>
          <a-option value="ssh">{{ $t('ssl.deployTargets.type.ssh') }}</a-option>
          <a-option value="aliyun_cas">{{ $t('ssl.deployTargets.type.aliyunCas') }}</a-option>
        </a-select>
      </a-form-item>

      <!-- Local Config -->
      <template v-if="formData.type === 'local'">
        <a-divider>{{ $t('ssl.deployTargets.type.local') }}</a-divider>
        
        <a-form-item field="config.base_path" :label="$t('ssl.deployTargets.local.basePath')">
          <a-input
            v-model="formData.config.base_path"
            :placeholder="$t('ssl.deployTargets.local.basePath.placeholder')"
            allow-clear
          />
        </a-form-item>

        <a-form-item field="config.fullchain_name" :label="$t('ssl.deployTargets.local.fullchainName')">
          <a-input
            v-model="formData.config.fullchain_name"
            :placeholder="$t('ssl.deployTargets.local.fullchainName.placeholder')"
            allow-clear
          />
        </a-form-item>

        <a-form-item field="config.privkey_name" :label="$t('ssl.deployTargets.local.privkeyName')">
          <a-input
            v-model="formData.config.privkey_name"
            :placeholder="$t('ssl.deployTargets.local.privkeyName.placeholder')"
            allow-clear
          />
        </a-form-item>

        <a-form-item field="config.file_mode" :label="$t('ssl.deployTargets.local.fileMode')">
          <a-input
            v-model="formData.config.file_mode"
            :placeholder="$t('ssl.deployTargets.local.fileMode.placeholder')"
            allow-clear
          />
        </a-form-item>

        <a-form-item field="config.reload_command" :label="$t('ssl.deployTargets.local.reloadCommand')">
          <a-input
            v-model="formData.config.reload_command"
            :placeholder="$t('ssl.deployTargets.local.reloadCommand.placeholder')"
            allow-clear
          />
        </a-form-item>
      </template>

      <!-- SSH Config -->
      <template v-if="formData.type === 'ssh'">
        <a-divider>{{ $t('ssl.deployTargets.type.ssh') }}</a-divider>

        <a-form-item field="config.server_id" :label="$t('ssl.deployTargets.ssh.server')">
          <a-select
            v-model="formData.config.server_id"
            :placeholder="$t('ssl.deployTargets.ssh.server.placeholder')"
            :loading="serversLoading"
            allow-search
            allow-clear
          >
            <a-option
              v-for="server in servers"
              :key="server.id"
              :value="server.id"
              :label="`${server.name} (${server.extranetHost || server.host})`"
            >
              {{ server.name }} ({{ server.extranetHost || server.host }})
            </a-option>
          </a-select>
        </a-form-item>

        <a-form-item field="config.remote_path" :label="$t('ssl.deployTargets.ssh.remotePath')">
          <a-input
            v-model="formData.config.remote_path"
            :placeholder="$t('ssl.deployTargets.ssh.remotePath.placeholder')"
            allow-clear
          />
        </a-form-item>

        <a-form-item field="config.fullchain_name" :label="$t('ssl.deployTargets.ssh.fullchainName')">
          <a-input
            v-model="formData.config.fullchain_name"
            :placeholder="$t('ssl.deployTargets.ssh.fullchainName.placeholder')"
            allow-clear
          />
        </a-form-item>

        <a-form-item field="config.privkey_name" :label="$t('ssl.deployTargets.ssh.privkeyName')">
          <a-input
            v-model="formData.config.privkey_name"
            :placeholder="$t('ssl.deployTargets.ssh.privkeyName.placeholder')"
            allow-clear
          />
        </a-form-item>

        <a-form-item field="config.file_mode" :label="$t('ssl.deployTargets.ssh.fileMode')">
          <a-input
            v-model="formData.config.file_mode"
            :placeholder="$t('ssl.deployTargets.ssh.fileMode.placeholder')"
            allow-clear
          />
        </a-form-item>

        <a-form-item field="config.reload_command" :label="$t('ssl.deployTargets.ssh.reloadCommand')">
          <a-input
            v-model="formData.config.reload_command"
            :placeholder="$t('ssl.deployTargets.ssh.reloadCommand.placeholder')"
            allow-clear
          />
        </a-form-item>
      </template>

      <!-- Aliyun CAS Config -->
      <template v-if="formData.type === 'aliyun_cas'">
        <a-divider>{{ $t('ssl.deployTargets.type.aliyunCas') }}</a-divider>

        <a-form-item field="config.dns_credential_id" :label="$t('ssl.deployTargets.aliyun.dnsCredential')">
          <a-select
            v-model="formData.config.dns_credential_id"
            :placeholder="$t('ssl.deployTargets.aliyun.dnsCredential.placeholder')"
            :loading="dnsCredentialsLoading"
            allow-search
            allow-clear
          >
            <a-option
              v-for="cred in dnsCredentials"
              :key="cred.id"
              :value="cred.id"
              :label="cred.name"
            >
              {{ cred.name }}
            </a-option>
          </a-select>
        </a-form-item>

        <a-form-item field="config.region" :label="$t('ssl.deployTargets.aliyun.region')">
          <a-select
            v-model="formData.config.region"
            :placeholder="$t('ssl.deployTargets.aliyun.region.placeholder')"
            allow-search
            allow-create
            allow-clear
          >
            <a-option value="cn-hangzhou">华东1（杭州）</a-option>
            <a-option value="cn-shanghai">华东2（上海）</a-option>
            <a-option value="cn-nanjing">华东5（南京-本地地域）</a-option>
            <a-option value="cn-fuzhou">华东6（福州-本地地域）</a-option>
            <a-option value="cn-qingdao">华北1（青岛）</a-option>
            <a-option value="cn-beijing">华北2（北京）</a-option>
            <a-option value="cn-zhangjiakou">华北3（张家口）</a-option>
            <a-option value="cn-huhehaote">华北5（呼和浩特）</a-option>
            <a-option value="cn-wulanchabu">华北6（乌兰察布）</a-option>
            <a-option value="cn-shenzhen">华南1（深圳）</a-option>
            <a-option value="cn-heyuan">华南2（河源）</a-option>
            <a-option value="cn-guangzhou">华南3（广州）</a-option>
            <a-option value="cn-chengdu">西南1（成都）</a-option>
            <a-option value="cn-hongkong">中国香港</a-option>
            <a-option value="ap-northeast-1">日本（东京）</a-option>
            <a-option value="ap-southeast-1">新加坡</a-option>
            <a-option value="ap-southeast-2">澳大利亚（悉尼）</a-option>
            <a-option value="ap-southeast-3">马来西亚（吉隆坡）</a-option>
            <a-option value="ap-southeast-5">印度尼西亚（雅加达）</a-option>
            <a-option value="ap-southeast-6">菲律宾（马尼拉）</a-option>
            <a-option value="ap-southeast-7">泰国（曼谷）</a-option>
            <a-option value="ap-south-1">印度（孟买）</a-option>
            <a-option value="us-west-1">美国（硅谷）</a-option>
            <a-option value="us-east-1">美国（弗吉尼亚）</a-option>
            <a-option value="eu-central-1">德国（法兰克福）</a-option>
            <a-option value="eu-west-1">英国（伦敦）</a-option>
            <a-option value="me-east-1">阿联酋（迪拜）</a-option>
          </a-select>
        </a-form-item>

        <a-form-item field="config.auto_deploy" :label="$t('ssl.deployTargets.aliyun.autoDeploy')">
          <a-switch v-model="formData.config.auto_deploy" />
        </a-form-item>
      </template>

      <a-form-item field="description" :label="$t('ssl.deployTargets.form.description')">
        <a-textarea
          v-model="formData.description"
          :placeholder="$t('ssl.deployTargets.form.description.placeholder')"
          :auto-size="{ minRows: 2, maxRows: 4 }"
          allow-clear
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('ssl.deployTargets.form.cancel') }}
        </a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('ssl.deployTargets.form.submit') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch, reactive } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import type {
  DeployTargetDTO,
  DeployTargetType,
  CreateDeployTargetRequest,
  UpdateDeployTargetRequest,
  LocalDeployConfig,
  SSHDeployConfig,
  AliyunCASDeployConfig,
  DnsCredentialDTO,
} from '@/api/ssl';
import { listDnsCredentials, listDeployTargets } from '@/api/ssl';
import { listServers, type ServerDTO } from '@/api/servers';

interface FormData {
  name: string;
  domain: string;
  type: DeployTargetType | '';
  description: string;
  config: {
    // Local config
    base_path?: string;
    // SSH config
    server_id?: number;
    remote_path?: string;
    // Common for local & ssh
    fullchain_name?: string;
    privkey_name?: string;
    file_mode?: string;
    reload_command?: string;
    // Aliyun CAS config
    dns_credential_id?: number;
    region?: string;
    auto_deploy?: boolean;
  };
}

const props = defineProps<{
  visible: boolean;
  editData?: DeployTargetDTO | null;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', data: CreateDeployTargetRequest | UpdateDeployTargetRequest): Promise<void>;
}>();

const { t } = useI18n();
const formRef = ref<FormInstance>();
const submitting = ref(false);

const isEdit = computed(() => !!props.editData);

const drawerVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

const formData = reactive<FormData>({
  name: '',
  domain: '',
  type: '',
  description: '',
  config: {},
});

// 下拉列表数据源
const servers = ref<ServerDTO[]>([]);
const serversLoading = ref(false);
const dnsCredentials = ref<DnsCredentialDTO[]>([]);
const dnsCredentialsLoading = ref(false);

// 自动生成文件名相关
let lastAutoGeneratedCrtName = '';
let lastAutoGeneratedKeyName = '';

// 生成文件名安全的域名字符串
const sanitizeDomainForFilename = (domain: string): string => {
  return domain
    .replace(/\*/g, '') // 去掉通配符
    .replace(/[^a-zA-Z0-9]/g, '_') // 非字母数字替换为_
    .replace(/_+/g, '_') // 合并多个_
    .replace(/^_|_$/g, ''); // trim首尾_
};

// 监听域名变化，自动生成文件名
watch(
  [() => formData.domain, () => formData.type],
  ([domain, type]) => {
    // 初始化默认路径
    if (type === 'local' && !formData.config.base_path) {
      formData.config.base_path = '/etc/nginx/conf.d/';
    }
    if (type === 'ssh' && !formData.config.remote_path) {
      formData.config.remote_path = '/etc/nginx/conf.d/';
    }
    
    // 自动生成文件名
    if ((type === 'local' || type === 'ssh') && domain) {
      const safeDomain = sanitizeDomainForFilename(domain);
      const newCrtName = `${safeDomain}.crt`;
      const newKeyName = `${safeDomain}.key`;
      
      // 只在当前值为空或等于上次自动生成值时更新
      if (!formData.config.fullchain_name || formData.config.fullchain_name === lastAutoGeneratedCrtName || formData.config.fullchain_name === 'fullchain.pem') {
        formData.config.fullchain_name = newCrtName;
        lastAutoGeneratedCrtName = newCrtName;
      }
      if (!formData.config.privkey_name || formData.config.privkey_name === lastAutoGeneratedKeyName || formData.config.privkey_name === 'privkey.pem') {
        formData.config.privkey_name = newKeyName;
        lastAutoGeneratedKeyName = newKeyName;
      }
      
      // 设置默认文件权限
      if (!formData.config.file_mode) {
        formData.config.file_mode = '0600';
      }
    }
  }
);

// 加载服务器列表
const loadServers = async () => {
  try {
    serversLoading.value = true;
    const { data } = await listServers({ enabled: true });
    servers.value = data.content || [];
  } catch (error) {
    console.error('加载服务器列表失败:', error);
  } finally {
    serversLoading.value = false;
  }
};

// 加载 DNS 凭证列表（过滤 alidns + enabled）
const loadDnsCredentials = async () => {
  try {
    dnsCredentialsLoading.value = true;
    const { data } = await listDnsCredentials(1, 100);
    dnsCredentials.value = (data.content || []).filter(
      (c) => c.provider === 'alidns' && c.status === 1
    );
  } catch (error) {
    console.error('加载 DNS 凭证列表失败:', error);
  } finally {
    dnsCredentialsLoading.value = false;
  }
};

// 监听抽屉打开，加载下拉数据
watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      loadServers();
      loadDnsCredentials();
    }
  }
);

const formRules = computed(() => ({
  name: [
    {
      required: true,
      message: t('ssl.deployTargets.validation.name.required'),
    },
  ],
  domain: [
    {
      required: true,
      message: t('ssl.deployTargets.validation.domain.required'),
    },
  ],
  type: [
    {
      required: true,
      message: t('ssl.deployTargets.validation.type.required'),
    },
  ],
  'config.base_path': formData.type === 'local' ? [
    {
      required: true,
      message: t('ssl.deployTargets.validation.basePath.required'),
    },
  ] : [],
  'config.server_id': formData.type === 'ssh' ? [
    {
      required: true,
      message: t('ssl.deployTargets.validation.serverId.required'),
    },
  ] : [],
  'config.remote_path': formData.type === 'ssh' ? [
    {
      required: true,
      message: t('ssl.deployTargets.validation.remotePath.required'),
    },
  ] : [],
  'config.dns_credential_id': formData.type === 'aliyun_cas' ? [
    {
      required: true,
      message: t('ssl.deployTargets.validation.dnsCredentialId.required'),
    },
  ] : [],
}));

const resetForm = () => {
  formData.name = '';
  formData.domain = '';
  formData.type = '';
  formData.description = '';
  formData.config = {};
  
  // 重置自动生成文件名追踪
  lastAutoGeneratedCrtName = '';
  lastAutoGeneratedKeyName = '';
  
  formRef.value?.clearValidate();
};

const handleTypeChange = () => {
  // 仅清理校验，不重置已填数据
  formRef.value?.clearValidate();
};

// 监听编辑数据变化，填充表单
watch(
  () => props.editData,
  (data) => {
    if (data) {
      formData.name = data.name;
      formData.domain = data.domain;
      formData.type = data.type;
      formData.description = data.description || '';
      
      // Parse config JSON
      try {
        const config = JSON.parse(data.config);
        formData.config = config;
        
        // 记录当前作为自动生成的值（避免误覆盖手动修改）
        if (data.type === 'local' || data.type === 'ssh') {
          lastAutoGeneratedCrtName = config.fullchain_name || '';
          lastAutoGeneratedKeyName = config.privkey_name || '';
        }
      } catch (error) {
        console.error('Failed to parse config JSON:', error);
        formData.config = {};
      }
    } else {
      resetForm();
    }
  },
  { immediate: true }
);

const handleCancel = () => {
  drawerVisible.value = false;
  resetForm();
};

const handleSubmit = async () => {
  const valid = await formRef.value?.validate();
  if (!valid) {
    try {
      submitting.value = true;
      
      // Prepare config based on type
      let config: any = {};
      
      if (formData.type === 'local') {
        config = {
          base_path: formData.config.base_path,
          fullchain_name: formData.config.fullchain_name,
          privkey_name: formData.config.privkey_name,
          file_mode: formData.config.file_mode || '0600',
          reload_command: formData.config.reload_command || '',
        };
      } else if (formData.type === 'ssh') {
        config = {
          server_id: Number(formData.config.server_id) || 0,
          remote_path: formData.config.remote_path,
          fullchain_name: formData.config.fullchain_name,
          privkey_name: formData.config.privkey_name,
          file_mode: formData.config.file_mode || '0600',
          reload_command: formData.config.reload_command || '',
        };
      } else if (formData.type === 'aliyun_cas') {
        config = {
          dns_credential_id: Number(formData.config.dns_credential_id) || 0,
          region: formData.config.region || 'cn-hangzhou',
          auto_deploy: formData.config.auto_deploy || false,
        };
      }
      
      // Construct request data
      const requestData: any = {
        name: formData.name,
        domain: formData.domain,
        description: formData.description || undefined,
        config: JSON.stringify(config),
      };
      
      if (!isEdit.value) {
        requestData.type = formData.type;
      }
      
      await emit('submit', requestData);
      drawerVisible.value = false;
      resetForm();
    } finally {
      submitting.value = false;
    }
  }
};
</script>

<style scoped lang="less">
:deep(.arco-form-item) {
  margin-bottom: 20px;
}
</style>

