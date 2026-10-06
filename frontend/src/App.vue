<script setup lang="ts">
import { computed, h } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import {
  NConfigProvider, NGlobalStyle, NLayout, NLayoutContent, NLayoutSider, NMenu, NTag,
  dateDeDE, darkTheme, deDE, useOsTheme, type MenuOption,
} from 'naive-ui'
import { navItems } from '@/router'
import { useBackendStatus } from '@/composables/useBackendStatus'

const { online, version } = useBackendStatus()
const os = useOsTheme()
const theme = computed(() => (os.value === 'dark' ? darkTheme : null))
const route = useRoute()
const menu: MenuOption[] = navItems.map((i) => ({
  key: i.name,
  label: () => h(RouterLink, { to: i.path }, () => i.label),
}))
</script>

<template>
  <n-config-provider :theme="theme" :locale="deDE" :date-locale="dateDeDE" class="root">
    <n-global-style />
    <n-layout has-sider class="root">
      <n-layout-sider bordered :width="220" content-style="display:flex;flex-direction:column;height:100%">
        <div class="brand">Kairo</div>
        <n-menu :value="route.name as string" :options="menu" />
        <div class="status">
          <n-tag :type="online ? 'success' : 'error'" round size="small">
            {{ online ? `Backend online${version ? ` (${version})` : ''}` : 'Backend offline' }}
          </n-tag>
        </div>
      </n-layout-sider>
      <n-layout-content content-style="padding:32px 40px;max-width:960px">
        <RouterView />
      </n-layout-content>
    </n-layout>
  </n-config-provider>
</template>

<style scoped>
.root { height: 100%; }
.brand { font-size: 20px; font-weight: 700; padding: 20px 24px 12px; color: #6f86ff; }
.status { margin-top: auto; padding: 16px; }
</style>
