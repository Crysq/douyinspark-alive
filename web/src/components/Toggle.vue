<template>
  <button class="toggle" :class="{ on: modelValue }" role="switch" :aria-checked="modelValue" @click="$emit('update:modelValue', !modelValue)" />
</template>

<script setup lang="ts">
defineProps<{ modelValue: boolean }>()
defineEmits<{ 'update:modelValue': [boolean] }>()
</script>
