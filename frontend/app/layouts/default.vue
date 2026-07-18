<template>
  <div class="flex h-screen overflow-hidden bg-default">
    <AppSidebar />
    <div class="flex-1 flex flex-col min-w-0">
      <AppTopbar />
      <main class="flex-1 overflow-y-auto px-4 py-5 sm:px-6 sm:py-6 lg:px-8 lg:py-7">
        <slot />
      </main>
    </div>
    <ConfirmDialog />
    <CommandPalette />
  </div>
</template>
