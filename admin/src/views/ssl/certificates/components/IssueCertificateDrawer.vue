<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="$t('ssl.certificates.issue')"
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
      <a-form-item field="name" :label="$t('ssl.certificates.form.name')">
        <a-input
          v-model="formData.name"
          :placeholder="$t('ssl.certificates.form.name.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="domain" :label="$t('ssl.certificates.form.domain')">
        <a-input
          v-model="formData.domain"
          :placeholder="$t('ssl.certificates.form.domain.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="email" :label="$t('ssl.certificates.form.email')">
        <a-input
          v-model="formData.email"
          :placeholder="$t('ssl.certificates.form.email.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="dns_credential_id" :label="$t('ssl.certificates.form.dnsCredentialId')">
        <a-select
          v-model="formData.dns_credential_id"
          :placeholder="$t('ssl.certificates.form.dnsCredentialId.placeholder')"
          :loading="dnsCredentialsLoading"
          allow-search
        >
          <a-option
            v-for="cred in dnsCredentials"
            :key="cred.id"
            :value="cred.id"
            :label="cred.name"
          >
            {{ cred.name }} ({{ cred.provider }})
          </a-option>
        </a-select>
      </a-form-item>

      <a-form-item field="renew_before_days" :label="$t('ssl.certificates.form.renewBeforeDays')">
        <a-input-number
          v-model="formData.renew_before_days"
          :placeholder="$t('ssl.certificates.form.renewBeforeDays.placeholder')"
          :min="1"
          :max="90"
          style="width: 100%"
        />
      </a-form-item>

      <a-form-item field="auto_renew" :label="$t('ssl.certificates.form.autoRenew')">
        <a-switch v-model="formData.auto_renew" />
      </a-form-item>

      <a-form-item field="auto_deploy" :label="$t('ssl.certificates.form.autoDeploy')">
        <a-switch v-model="formData.auto_deploy" />
        <template #extra>
          <a-typography-text type="secondary" style="font-size: 12px">
            {{ $t('ssl.certificates.form.autoDeploy.tip') }}
          </a-typography-text>
        </template>
      </a-form-item>

      <a-form-item field="use_staging" :label="$t('ssl.certificates.form.useStaging')">
        <a-switch v-model="formData.use_staging" />
        <template #extra>
          <a-typography-text type="secondary" style="font-size: 12px">
            {{ $t('ssl.certificates.form.useStaging.tip') }}
          </a-typography-text>
        </template>
      </a-form-item>

      <a-form-item field="description" :label="$t('ssl.certificates.form.description')">
        <a-textarea
          v-model="formData.description"
          :placeholder="$t('ssl.certificates.form.description.placeholder')"
          :auto-size="{ minRows: 2, maxRows: 4 }"
          allow-clear
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('ssl.certificates.form.cancel') }}
        </a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('ssl.certificates.form.submit') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import {
  issueCertificate,
  listDnsCredentials,
  type IssueCertificateRequest,
  type DnsCredentialDTO,
} from '@/api/ssl';

interface FormData {
  name: string;
  domain: string;
  email: string;
  dns_credential_id: number | undefined;
  renew_before_days: number;
  auto_renew: boolean;
  auto_deploy: boolean;
  use_staging: boolean;
  description: string;
}

const props = defineProps<{
  visible: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit'): void;
}>();

const { t } = useI18n();
const formRef = ref<FormInstance>();
const submitting = ref(false);

const drawerVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

const formData = ref<FormData>({
  name: '',
  domain: '',
  email: '',
  dns_credential_id: undefined,
  renew_before_days: 30,
  auto_renew: true,
  auto_deploy: true,
  use_staging: false,
  description: '',
});
const dnsCredentials = ref<DnsCredentialDTO[]>([]);
const dnsCredentialsLoading = ref(false);

const formRules = {
  domain: [
    {
      required: true,
      message: t('ssl.certificates.validation.domain.required'),
    },
  ],
  email: [
    {
      required: true,
      message: t('ssl.certificates.validation.email.required'),
    },
    {
      type: 'email',
      message: t('ssl.certificates.validation.email.format'),
    },
  ],
  dns_credential_id: [
    {
      required: true,
      message: t('ssl.certificates.validation.dnsCredentialId.required'),
    },
  ],
};

const resetForm = () => {
  formData.value = {
    name: '',
    domain: '',
    email: '',
    dns_credential_id: undefined,
    renew_before_days: 30,
    auto_renew: true,
    auto_deploy: true,
    use_staging: false,
    description: '',
  };
  formRef.value?.clearValidate();
};

// 加载 DNS 凭证
const fetchDnsCredentials = async () => {
  try {
    dnsCredentialsLoading.value = true;
    const { data } = await listDnsCredentials(1, 100);
    dnsCredentials.value = (data.content || []).filter(c => c.status === 1);
  } catch (error) {
    // Error handled by interceptor
  } finally {
    dnsCredentialsLoading.value = false;
  }
};

// 监听抽屉打开，加载数据
watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      fetchDnsCredentials();
    }
  }
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
      
      const requestData: IssueCertificateRequest = {
        domain: formData.value.domain,
        email: formData.value.email,
        dns_credential_id: formData.value.dns_credential_id!,
        renew_before_days: formData.value.renew_before_days,
        auto_renew: formData.value.auto_renew,
        auto_deploy: formData.value.auto_deploy,
        use_staging: formData.value.use_staging,
        description: formData.value.description || undefined,
      };

      // Only include name if it's not empty
      if (formData.value.name.trim()) {
        requestData.name = formData.value.name;
      }

      await issueCertificate(requestData);
      emit('submit');
      drawerVisible.value = false;
      resetForm();
    } finally {
      submitting.value = false;
    }
  }
};

onMounted(() => {
  if (props.visible) {
    fetchDnsCredentials();
  }
});
</script>

<style scoped lang="less">
:deep(.arco-form-item) {
  margin-bottom: 20px;
}
</style>

