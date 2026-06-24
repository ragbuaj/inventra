<template>
  <div class="flex h-screen overflow-hidden bg-muted">
    <AppSidebar />
    <div class="flex-1 flex flex-col min-w-0">
      <AppTopbar />
      <main class="flex-1 overflow-y-auto p-6">
        <slot />
      </main>
    </div>
    <ConfirmDialog />
  </div>
</template>
