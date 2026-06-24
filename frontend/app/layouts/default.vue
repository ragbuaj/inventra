<template>
  <div class="flex h-screen overflow-hidden bg-default">
    <AppSidebar />
    <div class="flex-1 flex flex-col min-w-0">
      <AppTopbar />
      <main class="flex-1 overflow-y-auto px-8 py-7">
        <slot />
      </main>
    </div>
    <ConfirmDialog />
  </div>
</template>
