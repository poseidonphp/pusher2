<script setup lang="ts">
import {ref} from "vue";

const darkmode = ref(false)
</script>

<template>
  <div :class="{'dark': darkmode}">
    <router-view />
  </div>

</template>

<style scoped>

</style>
