<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="isEdit ? $t('ssl.dnsCredentials.edit') : $t('ssl.dnsCredentials.create')"
    :width="500"
    @cancel="handleCancel"
    @before-ok="handleSubmit"
  >
    <a-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      layout="vertical"
    >
      <a-form-item field="name" :label="$t('ssl.dnsCredentials.form.name')">
        <a-input
          v-model="formData.name"
          :placeholder="$t('ssl.dnsCredentials.form.name.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="provider" :label="$t('ssl.dnsCredentials.form.provider')">
        <a-select
          v-model="formData.provider"
          :placeholder="$t('ssl.dnsCredentials.form.provider.placeholder')"
          :disabled="isEdit"
        >
          <a-option value="alidns">阿里云 DNS (alidns)</a-option>
          <a-option value="tencentcloud">腾讯云 DNS (tencentcloud)</a-option>
          <a-option value="dnspod">DNSPod (dnspod)</a-option>
          <a-option value="cloudflare">Cloudflare (cloudflare)</a-option>
        </a-select>
      </a-form-item>

      <a-form-item field="access_key" :label="$t('ssl.dnsCredentials.form.accessKey')">
        <a-input
          v-model="formData.access_key"
          :placeholder="$t('ssl.dnsCredentials.form.accessKey.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="secret_key" :label="$t('ssl.dnsCredentials.form.secretKey')">
        <a-input-password
          v-model="formData.secret_key"
          :placeholder="isEdit ? $t('ssl.dnsCredentials.form.secretKey.edit.placeholder') : $t('ssl.dnsCredentials.form.secretKey.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="extra_config" :label="$t('ssl.dnsCredentials.form.extraConfig')">
        <a-textarea
          v-model="formData.extra_config"
          :placeholder="$t('ssl.dnsCredentials.form.extraConfig.placeholder')"
          :auto-size="{ minRows: 2, maxRows: 4 }"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="description" :label="$t('ssl.dnsCredentials.form.description')">
        <a-textarea
          v-model="formData.description"
          :placeholder="$t('ssl.dnsCredentials.form.description.placeholder')"
          :auto-size="{ minRows: 2, maxRows: 4 }"
          allow-clear
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('ssl.dnsCredentials.form.cancel') }}
        </a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('ssl.dnsCredentials.form.submit') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import type { DnsCredentialDTO, DnsProvider, CreateDnsCredentialRequest, UpdateDnsCredentialRequest } from '@/api/ssl';

interface FormData {
  name: string;
  provider: DnsProvider | '';
  access_key: string;
  secret_key: string;
  extra_config: string;
  description: string;
}

const props = defineProps<{
  visible: boolean;
  editData?: DnsCredentialDTO | null;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', data: CreateDnsCredentialRequest | UpdateDnsCredentialRequest): Promise<void>;
}>();

const { t } = useI18n();
const formRef = ref<FormInstance>();
const submitting = ref(false);

const isEdit = computed(() => !!props.editData);

const drawerVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

const formData = ref<FormData>({
  name: '',
  provider: '',
  access_key: '',
  secret_key: '',
  extra_config: '',
  description: '',
});

const formRules = {
  name: [
    {
      required: true,
      message: t('ssl.dnsCredentials.validation.name.required'),
    },
  ],
  provider: [
    {
      required: true,
      message: t('ssl.dnsCredentials.validation.provider.required'),
    },
  ],
  access_key: [
    {
      required: !isEdit.value,
      message: t('ssl.dnsCredentials.validation.accessKey.required'),
    },
  ],
  secret_key: [
    {
      required: !isEdit.value,
      message: t('ssl.dnsCredentials.validation.secretKey.required'),
    },
  ],
};

const resetForm = () => {
  formData.value = {
    name: '',
    provider: '',
    access_key: '',
    secret_key: '',
    extra_config: '',
    description: '',
  };
  formRef.value?.clearValidate();
};

// 监听编辑数据变化，填充表单
watch(
  () => props.editData,
  (data) => {
    if (data) {
      formData.value = {
        name: data.name,
        provider: data.provider,
        access_key: '', // 编辑时不显示原始值
        secret_key: '', // 编辑时不显示原始值
        extra_config: data.extra_config || '',
        description: data.description || '',
      };
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
      
      // 构造请求数据
      const requestData: any = {
        name: formData.value.name,
        description: formData.value.description || undefined,
        extra_config: formData.value.extra_config || undefined,
      };

      if (isEdit.value) {
        // 编辑模式：只包含修改的字段
        if (formData.value.access_key) {
          requestData.access_key = formData.value.access_key;
        }
        if (formData.value.secret_key) {
          requestData.secret_key = formData.value.secret_key;
        }
      } else {
        // 创建模式：包含所有必填字段
        requestData.provider = formData.value.provider;
        requestData.access_key = formData.value.access_key;
        requestData.secret_key = formData.value.secret_key;
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

