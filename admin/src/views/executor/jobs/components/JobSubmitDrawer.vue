<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="$t('executor.submit.title')"
    :width="520"
    @cancel="handleCancel"
  >
    <a-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      layout="vertical"
    >
      <a-form-item field="env" :label="$t('executor.submit.env')">
        <a-select
          v-model="formData.env"
          :placeholder="$t('executor.submit.env.placeholder')"
          allow-create
          style="width: 100%"
        >
          <a-option value="dev">dev</a-option>
          <a-option value="test">test</a-option>
          <a-option value="prod">prod</a-option>
        </a-select>
      </a-form-item>

      <a-form-item field="target_service" :label="$t('executor.submit.targetService')">
        <a-input
          v-model="formData.target_service"
          :placeholder="$t('executor.submit.targetService.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="method" :label="$t('executor.submit.method')">
        <a-input
          v-model="formData.method"
          :placeholder="$t('executor.submit.method.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="args_json" :label="$t('executor.submit.argsJson')">
        <a-textarea
          v-model="formData.args_json"
          :placeholder="$t('executor.submit.argsJson.placeholder')"
          :auto-size="{ minRows: 3, maxRows: 8 }"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="run_at" :label="$t('executor.submit.runAt')">
        <a-date-picker
          v-model="formData.run_at_date"
          show-time
          format="YYYY-MM-DD HH:mm:ss"
          :placeholder="$t('executor.submit.runAt.placeholder')"
          style="width: 100%"
          allow-clear
        />
      </a-form-item>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item field="max_attempts" :label="$t('executor.submit.maxAttempts')">
            <a-input-number
              v-model="formData.max_attempts"
              :placeholder="$t('executor.submit.maxAttempts.placeholder')"
              :min="1"
              :max="100"
              style="width: 100%"
            />
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item field="priority" :label="$t('executor.submit.priority')">
            <a-input-number
              v-model="formData.priority"
              :placeholder="$t('executor.submit.priority.placeholder')"
              style="width: 100%"
            />
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item field="dedup_key" :label="$t('executor.submit.dedupKey')">
        <a-input
          v-model="formData.dedup_key"
          :placeholder="$t('executor.submit.dedupKey.placeholder')"
          allow-clear
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">{{ $t('executor.submit.cancel') }}</a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('executor.submit.confirm') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import type { SubmitJobRequest } from '@/api/executor';

const props = defineProps<{
  visible: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', data: SubmitJobRequest): void;
}>();

const { t } = useI18n();
const formRef = ref<FormInstance>();
const submitting = ref(false);

const drawerVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
});

interface FormData {
  env: string;
  target_service: string;
  method: string;
  args_json: string;
  run_at_date: string | null;
  max_attempts: number | null;
  priority: number | null;
  dedup_key: string;
}

const defaultForm = (): FormData => ({
  env: 'dev',
  target_service: '',
  method: '',
  args_json: '',
  run_at_date: null,
  max_attempts: null,
  priority: null,
  dedup_key: '',
});

const formData = ref<FormData>(defaultForm());

const formRules = {
  env: [
    { required: true, message: t('executor.submit.env.required') },
  ],
  target_service: [
    { required: true, message: t('executor.submit.targetService.required') },
  ],
  method: [
    { required: true, message: t('executor.submit.method.required') },
  ],
  args_json: [
    {
      validator: (value: string, callback: (msg?: string) => void) => {
        if (!value) { callback(); return; }
        try { JSON.parse(value); callback(); } catch { callback(t('executor.submit.argsJson.invalid')); }
      },
    },
  ],
};

watch(
  () => props.visible,
  (val) => { if (!val) formData.value = defaultForm(); }
);

const handleCancel = () => {
  drawerVisible.value = false;
};

const handleSubmit = async () => {
  const valid = await formRef.value?.validate();
  if (!valid) {
    try {
      submitting.value = true;
      const req: SubmitJobRequest = {
        env: formData.value.env,
        target_service: formData.value.target_service,
        method: formData.value.method,
        args_json: formData.value.args_json || undefined,
        max_attempts: formData.value.max_attempts || undefined,
        priority: formData.value.priority ?? undefined,
        dedup_key: formData.value.dedup_key || undefined,
      };
      if (formData.value.run_at_date) {
        req.run_at = Math.floor(new Date(formData.value.run_at_date).getTime() / 1000);
      }
      emit('submit', req);
    } finally {
      submitting.value = false;
    }
  }
};

const setSubmitting = (val: boolean) => { submitting.value = val; };
const closeDrawer = () => { drawerVisible.value = false; };

defineExpose({ setSubmitting, closeDrawer });
</script>

<style scoped lang="less">
:deep(.arco-form-item) {
  margin-bottom: 16px;
}
</style>
