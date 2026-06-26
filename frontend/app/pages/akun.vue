<script setup lang="ts">
import type { AccountProfile, AccountSession, NotifPrefs } from '~/types'
import { passwordStrength } from '~/utils/passwordStrength'

const { t, setLocale, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const toast = useToast()
const colorMode = useColorMode()
const account = useAccount()

const tab = ref<'profil' | 'keamanan' | 'pref'>(['profil', 'keamanan', 'pref'].includes(route.query.tab as string) ? route.query.tab as 'profil' | 'keamanan' | 'pref' : 'profil')
watch(tab, v => router.replace({ query: { ...route.query, tab: v } }))

const loading = ref(true)
const profile = ref<AccountProfile | null>(null)
const fNama = ref('')
const fTelepon = ref('')
const nameErr = ref(false)
const isGoogle = computed(() => profile.value?.loginMethod === 'google')

// security — used by Keamanan tab (C4)
const oldPass = ref('')
const newPass = ref('')
const confirmPass = ref('')
const secErr = reactive<{ old?: boolean, newp?: boolean, confirm?: boolean }>({})
const strength = computed(() => passwordStrength(newPass.value))
const sessions = ref<AccountSession[]>([])

// preferences — used by Preferensi tab (C5)
const themePref = ref(colorMode.preference)
const notif = ref<NotifPrefs>(account.getNotifPrefs())

onMounted(async () => {
  const [p, s] = await Promise.all([account.getProfile(), account.listSessions()])
  profile.value = p
  fNama.value = p.nama
  fTelepon.value = p.telepon
  sessions.value = s
  loading.value = false
})

async function saveProfil() {
  nameErr.value = !fNama.value.trim()
  if (nameErr.value) return
  try {
    await account.updateProfile({ nama: fNama.value, telepon: fTelepon.value })
    toast.add({ title: t('account.toastProfilTitle'), description: t('account.toastProfilMsg'), color: 'success' })
  } catch {
    toast.add({ title: t('common.error'), color: 'error' })
  }
}

async function changePassword() {
  secErr.old = !oldPass.value
  secErr.newp = !newPass.value
  secErr.confirm = !confirmPass.value || confirmPass.value !== newPass.value
  if (secErr.old || secErr.newp || secErr.confirm) return
  await account.changePassword({ oldPass: oldPass.value, newPass: newPass.value, confirmPass: confirmPass.value })
  oldPass.value = ''
  newPass.value = ''
  confirmPass.value = ''
  toast.add({ title: t('account.toastPassTitle'), description: t('account.toastPassMsg'), color: 'success' })
}

async function logoutAll() {
  await account.logoutAllOthers()
  toast.add({ title: t('account.toastLogoutTitle'), description: t('account.toastLogoutMsg'), color: 'success' })
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
function setTheme(pref: 'light' | 'dark' | 'system') {
  themePref.value = pref
  colorMode.preference = pref
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
function toggleNotif(k: keyof NotifPrefs) {
  notif.value = { ...notif.value, [k]: !notif.value[k] }
  account.setNotifPrefs(notif.value)
}

const _setLocale = setLocale

const initials = computed(() => {
  const n = (profile.value?.nama ?? '').trim().split(/\s+/)
  return ((n[0]?.[0] ?? '') + (n[1]?.[0] ?? '')).toUpperCase() || '?'
})
const joinDateLabel = computed(() => {
  if (!profile.value) return ''
  return new Date(profile.value.joinDate).toLocaleDateString(locale.value === 'en' ? 'en-GB' : 'id-ID', { day: 'numeric', month: 'long', year: 'numeric' })
})

function strengthBarClass(i: number): string {
  if (i > strength.value.score) return 'bg-muted'
  if (strength.value.score === 1) return 'bg-error'
  if (strength.value.score === 2) return 'bg-warning'
  return 'bg-primary'
}

const strengthLabelClass = computed(() => {
  if (strength.value.score === 1) return 'text-error'
  if (strength.value.score === 2) return 'text-warning'
  if (strength.value.score >= 3) return 'text-primary'
  return 'text-muted'
})
</script>

<template>
  <div class="flex-1 overflow-y-auto px-7 py-[26px] pb-11">
    <div class="max-w-[760px] mx-auto">
      <!-- LOADING SKELETON -->
      <template v-if="loading">
        <div class="flex items-center gap-[18px] mb-6">
          <div class="w-20 h-20 rounded-full bg-muted animate-pulse flex-none" />
          <div class="flex-1 flex flex-col gap-[9px]">
            <div class="h-4 w-[38%] rounded-md bg-muted animate-pulse" />
            <div class="h-[11px] w-[55%] rounded-md bg-muted animate-pulse" />
          </div>
        </div>
        <div class="h-[38px] w-[320px] rounded-[9px] mb-5 bg-muted" />
        <div class="flex flex-col gap-4">
          <div class="h-16 rounded-[11px] bg-muted" />
          <div class="h-16 rounded-[11px] bg-muted" />
          <div class="h-[120px] rounded-[11px] bg-muted" />
        </div>
      </template>

      <template v-else>
        <!-- PROFILE HEADER -->
        <div class="flex items-center gap-[18px] flex-wrap mb-[22px]">
          <div class="relative flex-none">
            <div class="w-20 h-20 rounded-full bg-gradient-to-br from-primary to-primary/60 text-inverted flex items-center justify-center text-[28px] font-bold shadow-sm">
              {{ initials }}
            </div>
            <button
              type="button"
              :title="t('account.changePhoto')"
              class="absolute right-[-2px] bottom-[-2px] w-7 h-7 rounded-full bg-default border border-[var(--ui-border-strong)] text-muted flex items-center justify-center cursor-pointer shadow-sm hover:text-primary hover:border-primary"
            >
              <UIcon
                name="i-lucide-camera"
                class="size-[14px]"
              />
            </button>
          </div>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-[10px] flex-wrap">
              <h1 class="m-0 text-[22px] font-bold tracking-tight">
                {{ profile?.nama }}
              </h1>
              <span class="px-[11px] py-[3px] text-[12px] font-semibold rounded-full bg-warning/15 text-warning">
                {{ profile?.peran }}
              </span>
            </div>
            <div class="flex items-center gap-[14px] flex-wrap mt-[6px]">
              <span class="inline-flex items-center gap-[6px] text-[13px] text-muted">
                <UIcon
                  name="i-lucide-mail"
                  class="size-[14px]"
                />
                {{ profile?.email }}
              </span>
              <span class="inline-flex items-center gap-[6px] text-[13px] text-muted">
                <UIcon
                  name="i-lucide-building-2"
                  class="size-[14px]"
                />
                {{ profile?.kantor }}
              </span>
            </div>
          </div>
        </div>

        <!-- TABS BAR -->
        <div class="flex gap-1 border-b border-default mb-[22px]">
          <button
            type="button"
            class="inline-flex items-center gap-2 px-4 py-3 -mb-px text-[14px] font-medium bg-transparent border-none cursor-pointer transition-colors"
            :class="tab === 'profil' ? 'text-primary border-b-2 border-primary' : 'text-muted hover:text-default'"
            @click="tab = 'profil'"
          >
            <UIcon
              name="i-lucide-user"
              class="size-4"
            />
            {{ t('account.tabProfil') }}
          </button>
          <button
            type="button"
            class="inline-flex items-center gap-2 px-4 py-3 -mb-px text-[14px] font-medium bg-transparent border-none cursor-pointer transition-colors"
            :class="tab === 'keamanan' ? 'text-primary border-b-2 border-primary' : 'text-muted hover:text-default'"
            @click="tab = 'keamanan'"
          >
            <UIcon
              name="i-lucide-shield"
              class="size-4"
            />
            {{ t('account.tabKeamanan') }}
          </button>
          <button
            type="button"
            class="inline-flex items-center gap-2 px-4 py-3 -mb-px text-[14px] font-medium bg-transparent border-none cursor-pointer transition-colors"
            :class="tab === 'pref' ? 'text-primary border-b-2 border-primary' : 'text-muted hover:text-default'"
            @click="tab = 'pref'"
          >
            <UIcon
              name="i-lucide-settings-2"
              class="size-4"
            />
            {{ t('account.tabPref') }}
          </button>
        </div>

        <!-- TAB: PROFIL -->
        <div
          v-if="tab === 'profil'"
          class="flex flex-col gap-[18px]"
        >
          <!-- Avatar block -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]">
            <div class="text-[13px] font-semibold mb-[14px]">
              {{ t('account.secFoto') }}
            </div>
            <div class="flex items-center gap-4 flex-wrap">
              <div class="w-[60px] h-[60px] rounded-full bg-gradient-to-br from-primary to-primary/60 text-inverted flex items-center justify-center text-[22px] font-bold flex-none">
                {{ initials }}
              </div>
              <div class="flex gap-[9px] flex-wrap">
                <button
                  type="button"
                  class="inline-flex items-center gap-[6px] px-[13px] py-2 text-[13px] font-medium text-default bg-default border border-[var(--ui-border-strong)] rounded-[9px] cursor-pointer hover:bg-muted"
                >
                  <UIcon
                    name="i-lucide-upload"
                    class="size-[14px]"
                  />
                  {{ t('account.upload') }}
                </button>
                <button
                  type="button"
                  class="inline-flex items-center gap-[6px] px-[13px] py-2 text-[13px] font-medium text-error bg-default border border-[var(--ui-border-strong)] rounded-[9px] cursor-pointer hover:bg-error/10 hover:border-transparent"
                >
                  <UIcon
                    name="i-lucide-trash-2"
                    class="size-[14px]"
                  />
                  {{ t('account.remove') }}
                </button>
              </div>
              <span class="text-[12px] text-dimmed">{{ t('account.fotoHint') }}</span>
            </div>
          </div>

          <!-- Data Diri form -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]">
            <div class="text-[13px] font-semibold mb-4">
              {{ t('account.secDiri') }}
            </div>
            <div class="grid grid-cols-2 gap-4">
              <!-- Full Name -->
              <div>
                <label class="block text-[13px] font-medium mb-[6px]">
                  {{ t('account.lNama') }} <span class="text-error">*</span>
                </label>
                <UInput
                  v-model="fNama"
                  :class="nameErr ? 'ring-1 ring-error [&_input]:border-error' : ''"
                  size="md"
                />
                <div
                  v-if="nameErr"
                  class="mt-[6px] text-[12px] text-error"
                >
                  {{ t('account.required') }}
                </div>
              </div>
              <!-- Phone -->
              <div>
                <label class="block text-[13px] font-medium mb-[6px]">
                  {{ t('account.lTelepon') }}
                </label>
                <UInput
                  v-model="fTelepon"
                  type="tel"
                  placeholder="08xx-xxxx-xxxx"
                  size="md"
                />
              </div>
              <!-- Email (full width) -->
              <div class="col-span-2">
                <label class="block text-[13px] font-medium mb-[6px]">
                  {{ t('account.lEmail') }}
                </label>
                <UInput
                  :model-value="profile?.email ?? ''"
                  :disabled="isGoogle"
                  :class="isGoogle ? 'opacity-60 cursor-not-allowed' : ''"
                  size="md"
                />
                <div
                  v-if="isGoogle"
                  class="mt-[6px] flex items-center gap-[5px] text-[12px] text-dimmed"
                >
                  <UIcon
                    name="i-lucide-lock"
                    class="size-3"
                  />
                  {{ t('account.emailLockNote') }}
                </div>
              </div>
            </div>
          </div>

          <!-- Info Akun (read-only) -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]">
            <div class="text-[13px] font-semibold mb-1">
              {{ t('account.secAkun') }}
            </div>
            <div class="text-[12px] text-dimmed mb-[14px]">
              {{ t('account.secAkunHint') }}
            </div>
            <div class="grid grid-cols-2 gap-x-7 gap-y-[14px]">
              <div>
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.iPeran') }}
                </div>
                <div class="text-[14px] font-medium">
                  {{ profile?.peran }}
                </div>
              </div>
              <div>
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.iKantor') }}
                </div>
                <div class="text-[14px] font-medium">
                  {{ profile?.kantor }}
                </div>
              </div>
              <div>
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.iPegawai') }}
                </div>
                <div class="text-[14px] font-medium">
                  {{ profile?.pegawai }}
                </div>
              </div>
              <div>
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.iLogin') }}
                </div>
                <div class="inline-flex items-center gap-[6px] text-[13.5px] font-medium">
                  <UIcon
                    :name="isGoogle ? 'i-simple-icons-google' : 'i-lucide-mail'"
                    class="size-[14px]"
                  />
                  {{ isGoogle ? t('account.loginGoogle') : t('account.loginEmail') }}
                </div>
              </div>
              <div>
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.iJoin') }}
                </div>
                <div class="text-[14px] font-medium">
                  {{ joinDateLabel }}
                </div>
              </div>
            </div>
          </div>

          <!-- Save button -->
          <div class="flex justify-end gap-[10px]">
            <UButton
              color="primary"
              icon="i-lucide-save"
              size="md"
              @click="saveProfil"
            >
              {{ t('account.save') }}
            </UButton>
          </div>
        </div>

        <!-- TAB: KEAMANAN -->
        <div
          v-else-if="tab === 'keamanan'"
          class="flex flex-col gap-[18px]"
        >
          <!-- Change Password card (email login only) -->
          <div
            v-if="!isGoogle"
            class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]"
          >
            <div class="text-[13px] font-semibold mb-4">
              {{ t('account.secPassword') }}
            </div>
            <div class="flex flex-col gap-[15px] max-w-[420px]">
              <!-- Current password -->
              <div>
                <label class="block text-[13px] font-medium mb-[6px]">
                  {{ t('account.lOldPass') }} <span class="text-error">*</span>
                </label>
                <UInput
                  v-model="oldPass"
                  type="password"
                  placeholder="••••••••"
                  :class="secErr.old ? 'ring-1 ring-error [&_input]:border-error' : ''"
                  size="md"
                />
                <div
                  v-if="secErr.old"
                  class="mt-[6px] text-[12px] text-error"
                >
                  {{ t('account.required') }}
                </div>
              </div>

              <!-- New password + strength meter -->
              <div>
                <label class="block text-[13px] font-medium mb-[6px]">
                  {{ t('account.lNewPass') }} <span class="text-error">*</span>
                </label>
                <UInput
                  v-model="newPass"
                  type="password"
                  placeholder="••••••••"
                  :class="secErr.newp ? 'ring-1 ring-error [&_input]:border-error' : ''"
                  size="md"
                />
                <div
                  v-if="secErr.newp"
                  class="mt-[6px] text-[12px] text-error"
                >
                  {{ t('account.required') }}
                </div>
                <!-- Strength meter -->
                <div
                  v-if="newPass.length"
                  class="mt-[9px]"
                >
                  <div class="flex gap-[5px] mb-[5px]">
                    <div
                      v-for="i in 4"
                      :key="i"
                      class="flex-1 h-[5px] rounded-full transition-colors"
                      :class="strengthBarClass(i)"
                    />
                  </div>
                  <div
                    class="text-[11.5px] font-medium"
                    :class="strengthLabelClass"
                  >
                    {{ strength.labelKey ? t(strength.labelKey) : '' }}
                  </div>
                </div>
              </div>

              <!-- Confirm password -->
              <div>
                <label class="block text-[13px] font-medium mb-[6px]">
                  {{ t('account.lConfirmPass') }} <span class="text-error">*</span>
                </label>
                <UInput
                  v-model="confirmPass"
                  type="password"
                  placeholder="••••••••"
                  :class="secErr.confirm ? 'ring-1 ring-error [&_input]:border-error' : ''"
                  size="md"
                />
                <div
                  v-if="secErr.confirm"
                  class="mt-[6px] flex items-center gap-[5px] text-[12px] text-error"
                >
                  <UIcon
                    name="i-lucide-alert-circle"
                    class="size-[13px] flex-none"
                  />
                  {{ t('account.confirmMismatch') }}
                </div>
              </div>

              <!-- Submit -->
              <div class="flex justify-start">
                <UButton
                  color="primary"
                  icon="i-lucide-lock"
                  size="md"
                  @click="changePassword"
                >
                  {{ t('account.changePass') }}
                </UButton>
              </div>
            </div>
          </div>

          <!-- Google login info card -->
          <div
            v-else
            class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]"
          >
            <div class="text-[13px] font-semibold mb-[14px]">
              {{ t('account.secPassword') }}
            </div>
            <div class="flex gap-[13px] items-center p-[15px_16px] rounded-[11px] bg-info/10 border border-info/30">
              <span class="w-10 h-10 rounded-[10px] bg-default flex items-center justify-center flex-none shadow-sm">
                <UIcon
                  name="i-simple-icons-google"
                  class="size-5 text-info"
                />
              </span>
              <div>
                <div class="text-[13.5px] font-semibold text-info">
                  {{ t('account.googleTitle') }}
                </div>
                <div class="text-[12.5px] leading-relaxed text-muted mt-[2px]">
                  {{ t('account.googleNote') }}
                </div>
              </div>
            </div>
          </div>

          <!-- Sessions card -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
            <!-- Header row -->
            <div class="flex items-center justify-between gap-3 px-5 py-[15px] border-b border-default">
              <span class="text-[13px] font-semibold">{{ t('account.secSesi') }}</span>
              <button
                type="button"
                class="inline-flex items-center gap-[6px] px-3 py-[7px] text-[12.5px] font-medium text-error bg-default border border-[var(--ui-border-strong)] rounded-[8px] cursor-pointer hover:bg-error/10 hover:border-transparent"
                @click="logoutAll"
              >
                <UIcon
                  name="i-lucide-log-out"
                  class="size-[14px]"
                />
                {{ t('account.logoutAll') }}
              </button>
            </div>
            <!-- Session rows -->
            <div>
              <div
                v-for="s in sessions"
                :key="s.id"
                class="flex items-center gap-[13px] px-5 py-[13px] border-b border-default last:border-b-0"
              >
                <span class="w-9 h-9 rounded-[9px] bg-muted text-muted flex items-center justify-center flex-none">
                  <UIcon
                    :name="s.icon"
                    class="size-[17px]"
                  />
                </span>
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="text-[13.5px] font-semibold">{{ s.device }}</span>
                    <span
                      v-if="s.current"
                      class="px-2 py-[1px] text-[10px] font-semibold rounded-full bg-primary/10 text-primary"
                    >
                      {{ t('account.current') }}
                    </span>
                  </div>
                  <div class="text-[12px] text-muted mt-[1px]">
                    {{ s.meta }}
                  </div>
                </div>
                <button
                  v-if="!s.current"
                  type="button"
                  class="text-[12px] font-medium text-muted bg-transparent border-none cursor-pointer px-2 py-[5px] rounded-[7px] hover:bg-error/10 hover:text-error"
                  @click="async () => { await account.revokeSession(s.id); sessions = sessions.filter(x => x.id !== s.id) }"
                >
                  {{ t('account.revoke') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- TAB: PREFERENSI — template filled in C5 -->
        <div v-else-if="tab === 'pref'" />
      </template>
    </div>
  </div>
</template>
