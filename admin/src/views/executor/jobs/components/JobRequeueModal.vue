<template>
  <a-modal
    v-model:visible="modalVisible"
    :title="$t('executor.requeue.title')"
    :ok-loading="submitting"
    @before-ok="handleSubmit"
    @cancel="handleCancel"
  >
    <a-form :model="formData" layout="vertical">
      <a-form-item :label="$t('executor.requeue.runAt')">
        <a-date-picker
          v-model="formData.run_at_date"
          show-time
          format="YYYY-MM-DD HH:mm:ss"
          :placeholder="$t('executor.requeue.runAt.placeholder')"
          style="width: 100%"
          allow-clear
        />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';

const props = defineProps<{
  visible: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', runAt?: number): void;
}>();

const submitting = ref(false);
const formData = ref({ run_at_date: null as string | null });

const modalVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
});

watch(
  () => props.visible,
  (val) => { if (val) formData.value.run_at_date = null; }
);

const handleSubmit = async (done: (closed: boolean) => void) => {
  try {
    submitting.value = true;
    const runAt = formData.value.run_at_date
      ? Math.floor(new Date(formData.value.run_at_date).getTime() / 1000)
      : undefined;
    emit('submit', runAt);
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
