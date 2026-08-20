<template>
  <a-modal
    v-model:visible="modalVisible"
    :title="$t('executor.cleanup.title')"
    :ok-loading="submitting"
    @before-ok="handleSubmit"
    @cancel="handleCancel"
  >
    <a-alert type="warning" style="margin-bottom: 16px">
      {{ $t('executor.cleanup.description') }}
    </a-alert>

    <a-form :model="formData" layout="vertical">
      <a-form-item :label="$t('executor.cleanup.env')">
        <a-select
          v-model="formData.env"
          allow-create
          style="width: 100%"
        >
          <a-option value="dev">dev</a-option>
          <a-option value="test">test</a-option>
          <a-option value="prod">prod</a-option>
        </a-select>
      </a-form-item>
      <a-form-item :label="$t('executor.cleanup.succeededDays')">
        <a-input-number
          v-model="formData.succeeded_days"
          :placeholder="$t('executor.cleanup.succeededDays.placeholder')"
          :min="0"
          style="width: 100%"
        />
      </a-form-item>
      <a-form-item :label="$t('executor.cleanup.canceledDays')">
        <a-input-number
          v-model="formData.canceled_days"
          :placeholder="$t('executor.cleanup.canceledDays.placeholder')"
          :min="0"
          style="width: 100%"
        />
      </a-form-item>
      <a-form-item :label="$t('executor.cleanup.deadDays')">
        <a-input-number
          v-model="formData.dead_days"
          :placeholder="$t('executor.cleanup.deadDays.placeholder')"
          :min="0"
          style="width: 100%"
        />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import type { CleanupJobsRequest } from '@/api/executor';

const props = defineProps<{
  visible: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', data: CleanupJobsRequest): void;
}>();

const submitting = ref(false);

const formData = ref<CleanupJobsRequest>({
  env: 'dev',
  succeeded_days: 7,
  canceled_days: 30,
  dead_days: 90,
});

const modalVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
});

watch(
  () => props.visible,
  (val) => {
    if (val) {
      formData.value = { env: 'dev', succeeded_days: 7, canceled_days: 30, dead_days: 90 };
    }
  }
);

const handleSubmit = async (done: (closed: boolean) => void) => {
  try {
    submitting.value = true;
    emit('submit', { ...formData.value });
    done(true);
  } finally {
    submitting.value = false;
  }
};

const handleCancel = () => {
  modalVisible.value = false;
};

const setSubmitting = (val: boolean) => { submitting.value = val; };

defineExpose({ setSubmitting });
</script>

<style scoped lang="less">
:deep(.arco-form-item) {
  margin-bottom: 16px;
}
</style>
