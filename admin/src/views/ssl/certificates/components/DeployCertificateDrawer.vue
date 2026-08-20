<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="$t('ssl.certificates.deploy.title')"
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
      <a-alert style="margin-bottom: 20px">
        {{ $t('Certificate') }}: <strong>{{ certificate?.name }}</strong>
      </a-alert>

      <a-form-item field="target_ids" :label="$t('ssl.certificates.deploy.selectTargets')">
        <a-select
          v-model="formData.target_ids"
          :placeholder="$t('ssl.certificates.deploy.selectTargets.placeholder')"
          :loading="deployTargetsLoading"
          multiple
          allow-search
        >
          <a-option
            v-for="target in deployTargets"
            :key="target.id"
            :value="target.id"
            :label="target.name"
          >
            {{ target.name }} ({{ target.type }})
          </a-option>
        </a-select>
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('ssl.certificates.deploy.cancel') }}
        </a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('ssl.certificates.deploy.submit') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import {
  deployCertificate,
  listDeployTargets,
  type CertificateDTO,
  type DeployTargetDTO,
} from '@/api/ssl';

interface FormData {
  target_ids: number[];
}

const props = defineProps<{
  visible: boolean;
  certificate: CertificateDTO | null;
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
  target_ids: [],
});

const deployTargets = ref<DeployTargetDTO[]>([]);
const deployTargetsLoading = ref(false);

const formRules = {
  target_ids: [
    {
      required: true,
      message: t('ssl.certificates.validation.deployTargets.required'),
    },
  ],
};

const resetForm = () => {
  formData.value = {
    target_ids: [],
  };
  formRef.value?.clearValidate();
};

// 加载部署目标
const fetchDeployTargets = async () => {
  try {
    deployTargetsLoading.value = true;
    const { data } = await listDeployTargets(1, 100);
    deployTargets.value = (data.content || []).filter(target => target.status === 1);
  } catch (error) {
    // Error handled by interceptor
  } finally {
    deployTargetsLoading.value = false;
  }
};

// 监听抽屉打开，加载数据
watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      fetchDeployTargets();
    } else {
      resetForm();
    }
  }
);

const handleCancel = () => {
  drawerVisible.value = false;
  resetForm();
};

const handleSubmit = async () => {
  const valid = await formRef.value?.validate();
  if (!valid && props.certificate) {
    try {
      submitting.value = true;
      
      await deployCertificate(props.certificate.id, {
        target_ids: formData.value.target_ids,
      });
      
      emit('submit');
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

