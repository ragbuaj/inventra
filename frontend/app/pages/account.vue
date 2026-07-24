<script setup lang="ts">
import type { AccountProfile, AccountSession, NotifPrefs } from '~/types'

const { t, setLocale, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const toast = useToast()
const colorMode = useColorMode()
const account = useAccount()

useHead({ title: t('account.title') })

const tab = ref<'profile' | 'security' | 'preferences'>(['profile', 'security', 'preferences'].includes(route.query.tab as string) ? route.query.tab as 'profile' | 'security' | 'preferences' : 'profile')
watch(tab, v => router.replace({ query: { ...route.query, tab: v } }))

const loading = ref(true)
const profile = ref<AccountProfile | null>(null)
const fNama = ref('')
const fTelepon = ref('')
const nameErr = ref(false)
const isGoogle = computed(() => profile.value?.loginMethod === 'google')

// Employee master-data status badge tone. Unknown/empty falls through to the
// neutral tone; the template renders a dash instead of a badge in that case.
const EMPLOYEE_STATUS_COLORS = {
  active: 'success',
  inactive: 'neutral',
  suspended: 'warning'
} as const
const employeeStatusColor = computed(
  () => EMPLOYEE_STATUS_COLORS[profile.value?.statusPegawai as keyof typeof EMPLOYEE_STATUS_COLORS] ?? 'neutral'
)

// Avatar — the image is fetched as a blob (the endpoint is authenticated), so
// the object URL must be revoked whenever it is replaced or the page unmounts,
// otherwise each upload leaks the previous blob.
const avatarUrl = ref<string | null>(null)
const avatarBusy = ref(false)
const avatarInput = ref<HTMLInputElement | null>(null)

function setAvatarUrl(next: string | null) {
  if (avatarUrl.value) URL.revokeObjectURL(avatarUrl.value)
  avatarUrl.value = next
}
onBeforeUnmount(() => setAvatarUrl(null))

async function refreshAvatar() {
  setAvatarUrl(profile.value?.hasAvatar ? await account.getAvatarObjectURL() : null)
}

function openAvatarPicker() {
  avatarInput.value?.click()
}

async function onAvatarSelected(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  // Reset immediately so re-picking the same file still fires a change event.
  input.value = ''
  if (!file) return
  avatarBusy.value = true
  try {
    profile.value = await account.uploadAvatar(file)
    await refreshAvatar()
    toast.add({ title: t('account.toastAvatarTitle'), description: t('account.toastAvatarMsg'), color: 'success' })
  } catch (err) {
    // Client-side validation throws a translatable key; the API path surfaces a
    // server message (413/415) via extractApiError.
    const msg = err instanceof Error && err.message.startsWith('account.') ? t(err.message) : extractApiError(err)
    toast.add({ title: t('common.error'), description: msg, color: 'error' })
  } finally {
    avatarBusy.value = false
  }
}

async function removeAvatar() {
  avatarBusy.value = true
  try {
    profile.value = await account.removeAvatar()
    setAvatarUrl(null)
    toast.add({ title: t('account.toastAvatarRemovedTitle'), color: 'success' })
  } catch (err) {
    toast.add({ title: t('common.error'), description: extractApiError(err), color: 'error' })
  } finally {
    avatarBusy.value = false
  }
}

// Profil tab — view/edit toggle (defaults read-only; Edit snapshots current
// values so Batal can revert without a re-fetch).
const editing = ref(false)
const savingProfile = ref(false)
let profileSnapshot: { nama: string, telepon: string } | null = null

// "Ubah Email" modal — request/sent two-step flow (Task 18).
const emailModalOpen = ref(false)
const newEmailInput = ref('')
const currentPasswordInput = ref('')
const newEmailErr = ref(false)
const emailApiErr = ref('')
const emailSent = ref(false)
const emailLoading = ref(false)
const emailCooldown = useResendCooldown(30)

// "Ganti Password" modal — request/sent two-step flow (Task 19). Mirrors the
// "Ubah Email" modal above: verifies the current password, then emails a
// reset link (the backend's password/change-request → reset-password flow)
// rather than changing the password inline — no logout on success.
const pwModalOpen = ref(false)
const pwCurrentInput = ref('')
const pwApiErr = ref('')
const pwSent = ref(false)
const pwLoading = ref(false)
const pwCooldown = useResendCooldown(30)
const sessions = ref<AccountSession[]>([])

// preferences — used by Preferensi tab (C5)
const themePref = ref(colorMode.preference)
watch(() => colorMode.preference, (v) => {
  themePref.value = v
})
const notif = ref<NotifPrefs>(account.getNotifPrefs())

onMounted(async () => {
  // The device-session list is supplementary: a failure to load it must never
  // block the whole account page (profile / security / preferences), so it is
  // fetched with its own catch rather than inside the page-critical Promise.all.
  const p = await account.getProfile()
  profile.value = p
  fNama.value = p.nama
  fTelepon.value = p.telepon
  loading.value = false
  // Supplementary, like the session list: a failed avatar fetch degrades to
  // initials and must never block the page.
  await refreshAvatar()
  sessions.value = await account.listSessions().catch(() => [])
})

function startEdit() {
  profileSnapshot = { nama: fNama.value, telepon: fTelepon.value }
  nameErr.value = false
  editing.value = true
}

function cancelEdit() {
  if (profileSnapshot) {
    fNama.value = profileSnapshot.nama
    fTelepon.value = profileSnapshot.telepon
  }
  nameErr.value = false
  editing.value = false
}

async function saveProfil() {
  nameErr.value = !fNama.value.trim()
  if (nameErr.value) return
  savingProfile.value = true
  try {
    // Adopt the server's response so the read-only detail below the form (and
    // the header name) reflects what was actually persisted.
    profile.value = await account.updateProfile({ nama: fNama.value, telepon: fTelepon.value })
    editing.value = false
    toast.add({ title: t('account.toastProfileTitle'), description: t('account.toastProfileMsg'), color: 'success' })
  } catch {
    toast.add({ title: t('common.error'), color: 'error' })
  } finally {
    savingProfile.value = false
  }
}

function isValidEmail(v: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v.trim())
}

// The backend returns {error: "..."} for both the 400 (wrong password) and
// 409 (email in use / same email) cases; ofetch/$fetch surfaces the parsed
// body on `err.data` (see useApiClient's doFetch).
function extractApiError(err: unknown): string {
  const data = (err as { data?: unknown } | undefined)?.data
  if (data && typeof data === 'object' && 'error' in data && typeof (data as { error?: unknown }).error === 'string') {
    return (data as { error: string }).error
  }
  if (typeof data === 'string' && data) return data
  return t('common.error')
}

function openEmailModal() {
  newEmailInput.value = ''
  currentPasswordInput.value = ''
  newEmailErr.value = false
  emailApiErr.value = ''
  emailSent.value = false
  emailCooldown.reset()
  emailModalOpen.value = true
}

async function submitEmailChange() {
  if (emailSent.value) {
    emailModalOpen.value = false
    return
  }
  newEmailErr.value = !isValidEmail(newEmailInput.value)
  if (newEmailErr.value) return
  emailApiErr.value = ''
  emailLoading.value = true
  try {
    await account.requestEmailChange(newEmailInput.value.trim(), currentPasswordInput.value)
    emailSent.value = true
    emailCooldown.start()
  } catch (err) {
    emailApiErr.value = extractApiError(err)
  } finally {
    emailLoading.value = false
  }
}

async function resendEmailChange() {
  if (!emailCooldown.canResend.value) return
  emailApiErr.value = ''
  emailLoading.value = true
  try {
    await account.requestEmailChange(newEmailInput.value.trim(), currentPasswordInput.value)
    emailCooldown.start()
  } catch (err) {
    emailApiErr.value = extractApiError(err)
  } finally {
    emailLoading.value = false
  }
}

function openPasswordModal() {
  pwCurrentInput.value = ''
  pwApiErr.value = ''
  pwSent.value = false
  pwCooldown.reset()
  pwModalOpen.value = true
}

async function submitPasswordChange() {
  if (pwSent.value) {
    pwModalOpen.value = false
    return
  }
  pwApiErr.value = ''
  pwLoading.value = true
  try {
    await account.requestPasswordChange(pwCurrentInput.value)
    pwSent.value = true
    pwCooldown.start()
  } catch (err) {
    pwApiErr.value = extractApiError(err)
  } finally {
    pwLoading.value = false
  }
}

async function resendPasswordChange() {
  if (!pwCooldown.canResend.value) return
  pwApiErr.value = ''
  pwLoading.value = true
  try {
    await account.requestPasswordChange(pwCurrentInput.value)
    pwCooldown.start()
  } catch (err) {
    pwApiErr.value = extractApiError(err)
  } finally {
    pwLoading.value = false
  }
}

async function logoutAll() {
  await account.logoutAllOthers()
  // Re-fetch so the list collapses to just the current device.
  sessions.value = await account.listSessions()
  toast.add({ title: t('account.toastLogoutTitle'), description: t('account.toastLogoutMsg'), color: 'success' })
}

function setTheme(pref: 'light' | 'dark' | 'system') {
  themePref.value = pref
  colorMode.preference = pref
}

function toggleNotif(k: keyof NotifPrefs) {
  notif.value = { ...notif.value, [k]: !notif.value[k] }
  account.setNotifPrefs(notif.value)
}

const notifItems = computed(() => [
  { key: 'approval' as keyof NotifPrefs, icon: 'i-lucide-check-circle', label: t('account.notifApproval'), desc: t('account.notifApprovalDesc'), iconBg: 'bg-primary/10', iconColor: 'text-primary' },
  { key: 'maint' as keyof NotifPrefs, icon: 'i-lucide-wrench', label: t('account.notifMaint'), desc: t('account.notifMaintDesc'), iconBg: 'bg-warning/10', iconColor: 'text-warning' },
  { key: 'assign' as keyof NotifPrefs, icon: 'i-lucide-package', label: t('account.notifAssign'), desc: t('account.notifAssignDesc'), iconBg: 'bg-info/10', iconColor: 'text-info' }
])

const initials = computed(() => {
  const n = (profile.value?.nama ?? '').trim().split(/\s+/)
  return ((n[0]?.[0] ?? '') + (n[1]?.[0] ?? '')).toUpperCase() || '?'
})
const joinDateLabel = computed(() => {
  if (!profile.value) return ''
  return new Date(profile.value.joinDate).toLocaleDateString(locale.value === 'en' ? 'en-GB' : 'id-ID', { day: 'numeric', month: 'long', year: 'numeric' })
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
            <img
              v-if="avatarUrl"
              :src="avatarUrl"
              :alt="profile?.nama ?? ''"
              class="w-20 h-20 rounded-full object-cover shadow-sm"
              data-testid="header-avatar-image"
            >
            <div
              v-else
              class="w-20 h-20 rounded-full bg-gradient-to-br from-primary to-primary/60 text-inverted flex items-center justify-center text-[28px] font-bold shadow-sm"
              data-testid="header-avatar-initials"
            >
              {{ initials }}
            </div>
            <button
              type="button"
              :title="t('account.changePhoto')"
              :disabled="avatarBusy"
              data-testid="header-change-photo"
              class="absolute right-[-2px] bottom-[-2px] w-7 h-7 rounded-full bg-default border border-[var(--ui-border-strong)] text-muted flex items-center justify-center cursor-pointer shadow-sm hover:text-primary hover:border-primary disabled:opacity-50 disabled:cursor-not-allowed"
              @click="openAvatarPicker"
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
                {{ profile?.kantor || '—' }}
              </span>
            </div>
          </div>
        </div>

        <!-- TABS BAR -->
        <div class="flex gap-1 border-b border-default mb-[22px]">
          <button
            type="button"
            class="flex-1 sm:flex-none inline-flex items-center justify-center sm:justify-start gap-2 px-2 sm:px-4 py-3 -mb-px text-[14px] font-medium whitespace-nowrap bg-transparent border-none cursor-pointer transition-colors"
            :class="tab === 'profile' ? 'text-primary border-b-2 border-primary' : 'text-muted hover:text-default'"
            @click="tab = 'profile'"
          >
            <UIcon
              name="i-lucide-user"
              class="size-4"
            />
            {{ t('account.tabProfile') }}
          </button>
          <button
            type="button"
            class="flex-1 sm:flex-none inline-flex items-center justify-center sm:justify-start gap-2 px-2 sm:px-4 py-3 -mb-px text-[14px] font-medium whitespace-nowrap bg-transparent border-none cursor-pointer transition-colors"
            :class="tab === 'security' ? 'text-primary border-b-2 border-primary' : 'text-muted hover:text-default'"
            @click="tab = 'security'"
          >
            <UIcon
              name="i-lucide-shield"
              class="size-4"
            />
            {{ t('account.tabSecurity') }}
          </button>
          <button
            type="button"
            class="flex-1 sm:flex-none inline-flex items-center justify-center sm:justify-start gap-2 px-2 sm:px-4 py-3 -mb-px text-[14px] font-medium whitespace-nowrap bg-transparent border-none cursor-pointer transition-colors"
            :class="tab === 'preferences' ? 'text-primary border-b-2 border-primary' : 'text-muted hover:text-default'"
            @click="tab = 'preferences'"
          >
            <UIcon
              name="i-lucide-settings-2"
              class="size-4"
            />
            {{ t('account.tabPreferences') }}
          </button>
        </div>

        <!-- TAB: PROFIL -->
        <div
          v-if="tab === 'profile'"
          class="flex flex-col gap-[18px]"
        >
          <!-- Avatar block -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]">
            <div class="text-[13px] font-semibold mb-[14px]">
              {{ t('account.secPhoto') }}
            </div>
            <div class="flex items-center gap-4 flex-wrap">
              <img
                v-if="avatarUrl"
                :src="avatarUrl"
                :alt="profile?.nama ?? ''"
                class="w-[60px] h-[60px] rounded-full object-cover flex-none"
                data-testid="avatar-image"
              >
              <div
                v-else
                class="w-[60px] h-[60px] rounded-full bg-gradient-to-br from-primary to-primary/60 text-inverted flex items-center justify-center text-[22px] font-bold flex-none"
                data-testid="avatar-initials"
              >
                {{ initials }}
              </div>
              <!-- The real file input stays visually hidden; both the buttons
                   here and the header camera icon trigger it. -->
              <input
                ref="avatarInput"
                type="file"
                accept="image/jpeg,image/png,.jpg,.jpeg,.png"
                class="hidden"
                data-testid="avatar-input"
                @change="onAvatarSelected"
              >
              <div class="flex gap-[9px] flex-wrap">
                <button
                  type="button"
                  :disabled="avatarBusy"
                  data-testid="avatar-upload"
                  class="inline-flex items-center gap-[6px] px-[13px] py-2 text-[13px] font-medium text-default bg-default border border-[var(--ui-border-strong)] rounded-[9px] cursor-pointer hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed"
                  @click="openAvatarPicker"
                >
                  <UIcon
                    :name="avatarBusy ? 'i-lucide-loader-circle' : 'i-lucide-upload'"
                    :class="['size-[14px]', avatarBusy && 'animate-spin']"
                  />
                  {{ t('account.upload') }}
                </button>
                <!-- Deliberate, user-approved deviation from the mockup (which
                     always shows it): hidden until there is a photo to remove,
                     so the control is never a silent no-op. -->
                <button
                  v-if="profile?.hasAvatar"
                  type="button"
                  :disabled="avatarBusy"
                  data-testid="avatar-remove"
                  class="inline-flex items-center gap-[6px] px-[13px] py-2 text-[13px] font-medium text-error bg-default border border-[var(--ui-border-strong)] rounded-[9px] cursor-pointer hover:bg-error/10 hover:border-transparent disabled:opacity-50 disabled:cursor-not-allowed"
                  @click="removeAvatar"
                >
                  <UIcon
                    name="i-lucide-trash-2"
                    class="size-[14px]"
                  />
                  {{ t('account.remove') }}
                </button>
              </div>
              <span class="text-[12px] text-dimmed">{{ t('account.photoHint') }}</span>
            </div>
          </div>

          <!-- Data Diri form. The edit controls live in this card's header
               because editing only ever touches the fields inside it. -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]">
            <div class="flex items-start justify-between gap-4 mb-4">
              <!-- min-w-0 lets the hint wrap inside its own column instead of
                   pushing the edit controls onto a line of their own. -->
              <div class="min-w-0">
                <div class="text-[13px] font-semibold">
                  {{ t('account.secPersonal') }}
                </div>
                <div class="text-[12px] text-dimmed mt-[3px]">
                  {{ t('account.secPersonalHint') }}
                </div>
              </div>
              <div class="flex gap-[10px] flex-none">
                <template v-if="!editing">
                  <UButton
                    color="primary"
                    variant="outline"
                    icon="i-lucide-pencil"
                    size="sm"
                    data-testid="profile-edit"
                    @click="startEdit"
                  >
                    {{ t('account.edit') }}
                  </UButton>
                </template>
                <template v-else>
                  <UButton
                    color="neutral"
                    variant="ghost"
                    size="sm"
                    data-testid="profile-cancel"
                    @click="cancelEdit"
                  >
                    {{ t('account.cancel') }}
                  </UButton>
                  <UButton
                    color="primary"
                    icon="i-lucide-save"
                    size="sm"
                    :loading="savingProfile"
                    data-testid="profile-save"
                    @click="saveProfil"
                  >
                    {{ t('account.save') }}
                  </UButton>
                </template>
              </div>
            </div>
            <!-- One unified grid: the two self-editable fields sit alongside the
                 employee master-data fields, which are read-only in every state
                 (they are maintained on the Master Data Pegawai screen). Outside
                 edit mode nothing renders as an input — plain label/value rows,
                 matching the Informasi Akun card below. -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-7 gap-y-[14px]">
              <!-- Full Name -->
              <div>
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.lName') }} <span
                    v-if="editing"
                    class="text-error"
                  >*</span>
                </div>
                <UInput
                  v-if="editing"
                  v-model="fNama"
                  :class="nameErr ? 'ring-1 ring-error [&_input]:border-error' : ''"
                  data-testid="profile-nama"
                  size="md"
                />
                <div
                  v-else
                  class="text-[14px] font-medium"
                  data-testid="profile-nama"
                >
                  {{ profile?.nama || '—' }}
                </div>
                <div
                  v-if="nameErr"
                  class="mt-[6px] text-[12px] text-error"
                >
                  {{ t('account.required') }}
                </div>
              </div>
              <!-- Phone -->
              <div>
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.lPhone') }}
                </div>
                <UInput
                  v-if="editing"
                  v-model="fTelepon"
                  type="tel"
                  placeholder="08xx-xxxx-xxxx"
                  :disabled="!profile?.hasEmployee"
                  data-testid="profile-telepon"
                  size="md"
                />
                <div
                  v-else
                  class="text-[14px] font-medium"
                  data-testid="profile-telepon"
                >
                  {{ profile?.telepon || '—' }}
                </div>
                <div
                  v-if="!profile?.hasEmployee"
                  class="mt-[6px] text-[12px] text-dimmed"
                  data-testid="profile-telepon-hint"
                >
                  {{ t('account.phoneManagedNote') }}
                </div>
              </div>

              <!-- Employee master-data fields — read-only in every state. -->
              <template v-if="profile?.hasEmployee">
                <div data-testid="profile-employee-detail">
                  <div class="text-[12px] text-muted mb-[3px]">
                    {{ t('account.iEmployee') }}
                  </div>
                  <div
                    class="text-[14px] font-medium"
                    data-testid="profile-employee-name"
                  >
                    {{ profile?.pegawai || '—' }}
                  </div>
                </div>
                <div>
                  <div class="text-[12px] text-muted mb-[3px]">
                    {{ t('account.iEmployeeCode') }}
                  </div>
                  <div
                    class="text-[14px] font-medium"
                    data-testid="profile-employee-code"
                  >
                    {{ profile?.kodePegawai || '—' }}
                  </div>
                </div>
                <div>
                  <div class="text-[12px] text-muted mb-[3px]">
                    {{ t('account.iDepartment') }}
                  </div>
                  <div
                    class="text-[14px] font-medium"
                    data-testid="profile-department"
                  >
                    {{ profile?.departemen || '—' }}
                  </div>
                </div>
                <div>
                  <div class="text-[12px] text-muted mb-[3px]">
                    {{ t('account.iPosition') }}
                  </div>
                  <div
                    class="text-[14px] font-medium"
                    data-testid="profile-position"
                  >
                    {{ profile?.jabatan || '—' }}
                  </div>
                </div>
                <div>
                  <div class="text-[12px] text-muted mb-[3px]">
                    {{ t('account.iEmployeeStatus') }}
                  </div>
                  <UBadge
                    v-if="profile?.statusPegawai"
                    :color="employeeStatusColor"
                    variant="subtle"
                    size="sm"
                    data-testid="profile-employee-status"
                  >
                    {{ t(`account.status_${profile.statusPegawai}`) }}
                  </UBadge>
                  <div
                    v-else
                    class="text-[14px] font-medium"
                    data-testid="profile-employee-status"
                  >
                    —
                  </div>
                </div>
              </template>
              <div
                v-else
                class="col-span-full text-[13px] text-dimmed"
                data-testid="profile-no-employee"
              >
                {{ t('account.noEmployeeLinked') }}
              </div>
            </div>
          </div>

          <!-- Info Akun (read-only) -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]">
            <div class="text-[13px] font-semibold mb-1">
              {{ t('account.secAccount') }}
            </div>
            <div class="text-[12px] text-dimmed mb-[14px]">
              {{ t('account.secAccountHint') }}
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-7 gap-y-[14px]">
              <!-- Email lives here, not under Data Diri: it identifies the login
                   account rather than the person. Changing it is a verified flow
                   of its own, hence the dedicated button instead of the card's
                   edit mode. -->
              <div class="sm:col-span-2">
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.lEmail') }}
                </div>
                <div class="flex items-center gap-[10px] flex-wrap">
                  <span
                    class="text-[14px] font-medium"
                    data-testid="profile-email"
                  >{{ profile?.email }}</span>
                  <UButton
                    v-if="!isGoogle"
                    color="neutral"
                    variant="outline"
                    size="xs"
                    data-testid="profile-change-email"
                    @click="openEmailModal"
                  >
                    {{ t('account.changeEmail') }}
                  </UButton>
                </div>
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
              <div>
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.iRole') }}
                </div>
                <div class="text-[14px] font-medium">
                  {{ profile?.peran }}
                </div>
              </div>
              <div>
                <div class="text-[12px] text-muted mb-[3px]">
                  {{ t('account.iOffice') }}
                </div>
                <div class="text-[14px] font-medium">
                  {{ profile?.kantor || '—' }}
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

          <!-- "Ubah Email" modal -->
          <FormModal
            v-model:open="emailModalOpen"
            :title="t('account.changeEmail')"
            :loading="emailLoading"
            :hide-footer="emailSent"
            @submit="submitEmailChange"
          >
            <template v-if="!emailSent">
              <div class="flex flex-col gap-4">
                <UFormField :label="t('account.newEmail')">
                  <UInput
                    v-model="newEmailInput"
                    type="email"
                    class="w-full"
                    :class="newEmailErr ? 'ring-1 ring-error [&_input]:border-error' : ''"
                    data-testid="change-email-input"
                  />
                  <div
                    v-if="newEmailErr"
                    class="mt-[6px] text-[12px] text-error"
                  >
                    {{ t('account.invalidEmail') }}
                  </div>
                </UFormField>
                <UFormField :label="t('account.currentPassword')">
                  <UInput
                    v-model="currentPasswordInput"
                    type="password"
                    class="w-full"
                    data-testid="change-email-password"
                  />
                </UFormField>
                <div
                  v-if="emailApiErr"
                  class="text-[12px] text-error"
                  data-testid="change-email-error"
                >
                  {{ emailApiErr }}
                </div>
              </div>
            </template>
            <template v-else>
              <div class="flex flex-col gap-3">
                <UAlert
                  color="success"
                  variant="soft"
                  :title="t('account.emailVerifySent', { email: newEmailInput })"
                  data-testid="change-email-sent"
                />
                <div
                  v-if="emailApiErr"
                  class="text-[12px] text-error"
                  data-testid="change-email-error"
                >
                  {{ emailApiErr }}
                </div>
                <div class="flex justify-end gap-2">
                  <UButton
                    color="neutral"
                    variant="ghost"
                    data-testid="change-email-close"
                    @click="() => { emailModalOpen = false }"
                  >
                    {{ t('common.cancel') }}
                  </UButton>
                  <UButton
                    variant="soft"
                    :disabled="!emailCooldown.canResend.value || emailLoading"
                    :loading="emailLoading"
                    data-testid="change-email-resend"
                    @click="resendEmailChange"
                  >
                    {{ emailCooldown.canResend.value ? t('auth.forgotResend') : t('auth.forgotResendWait', { s: emailCooldown.remaining.value }) }}
                  </UButton>
                </div>
              </div>
            </template>
          </FormModal>
        </div>

        <!-- TAB: KEAMANAN -->
        <div
          v-else-if="tab === 'security'"
          class="flex flex-col gap-[18px]"
        >
          <!-- Change Password card (email login only) -->
          <div
            v-if="!isGoogle"
            class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]"
          >
            <div class="text-[13px] font-semibold mb-2">
              {{ t('account.secPassword') }}
            </div>
            <p class="text-[12.5px] text-muted mb-4 max-w-[420px]">
              {{ t('account.changePasswordDesc') }}
            </p>
            <UButton
              color="primary"
              icon="i-lucide-lock"
              size="md"
              data-testid="security-change-password"
              @click="openPasswordModal"
            >
              {{ t('account.changePassword') }}
            </UButton>
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
              <span class="text-[13px] font-semibold">{{ t('account.secSessions') }}</span>
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

          <!-- "Ganti Password" modal -->
          <FormModal
            v-model:open="pwModalOpen"
            :title="t('account.changePassword')"
            :loading="pwLoading"
            :hide-footer="pwSent"
            @submit="submitPasswordChange"
          >
            <template v-if="!pwSent">
              <div class="flex flex-col gap-4">
                <UFormField :label="t('account.currentPassword')">
                  <UInput
                    v-model="pwCurrentInput"
                    type="password"
                    class="w-full"
                    data-testid="change-password-current"
                  />
                </UFormField>
                <div
                  v-if="pwApiErr"
                  class="text-[12px] text-error"
                  data-testid="change-password-error"
                >
                  {{ pwApiErr }}
                </div>
              </div>
            </template>
            <template v-else>
              <div class="flex flex-col gap-3">
                <UAlert
                  color="success"
                  variant="soft"
                  :title="t('account.pwChangeSent')"
                  data-testid="change-password-sent"
                />
                <div
                  v-if="pwApiErr"
                  class="text-[12px] text-error"
                  data-testid="change-password-error"
                >
                  {{ pwApiErr }}
                </div>
                <div class="flex justify-end gap-2">
                  <UButton
                    color="neutral"
                    variant="ghost"
                    data-testid="change-password-close"
                    @click="() => { pwModalOpen = false }"
                  >
                    {{ t('common.cancel') }}
                  </UButton>
                  <UButton
                    variant="soft"
                    :disabled="!pwCooldown.canResend.value || pwLoading"
                    :loading="pwLoading"
                    data-testid="change-password-resend"
                    @click="resendPasswordChange"
                  >
                    {{ pwCooldown.canResend.value ? t('auth.forgotResend') : t('auth.forgotResendWait', { s: pwCooldown.remaining.value }) }}
                  </UButton>
                </div>
              </div>
            </template>
          </FormModal>
        </div>

        <!-- TAB: PREFERENSI -->
        <div
          v-else-if="tab === 'preferences'"
          class="flex flex-col gap-[18px]"
        >
          <!-- Tampilan card -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]">
            <div class="text-[13px] font-semibold mb-4">
              {{ t('account.secAppearance') }}
            </div>
            <div class="flex flex-col gap-[18px]">
              <!-- Language row -->
              <div class="flex items-center justify-between gap-4 flex-wrap">
                <div>
                  <div class="text-[14px] font-medium">
                    {{ t('account.lLanguage') }}
                  </div>
                  <div class="text-[12px] text-muted mt-[1px]">
                    {{ t('account.lLanguageHint') }}
                  </div>
                </div>
                <div class="flex gap-[3px] p-[3px] bg-muted rounded-[9px]">
                  <button
                    type="button"
                    class="px-[14px] py-[6px] text-[13px] font-semibold rounded-[7px] border-none cursor-pointer transition-colors"
                    :class="locale === 'id' ? 'bg-default text-default shadow-sm' : 'bg-transparent text-muted hover:text-default'"
                    @click="setLocale('id')"
                  >
                    Indonesia
                  </button>
                  <button
                    type="button"
                    class="px-[14px] py-[6px] text-[13px] font-semibold rounded-[7px] border-none cursor-pointer transition-colors"
                    :class="locale === 'en' ? 'bg-default text-default shadow-sm' : 'bg-transparent text-muted hover:text-default'"
                    @click="setLocale('en')"
                  >
                    English
                  </button>
                </div>
              </div>

              <!-- Divider -->
              <div class="h-px bg-default" />

              <!-- Theme row -->
              <div>
                <div class="mb-[11px]">
                  <div class="text-[14px] font-medium">
                    {{ t('account.lTheme') }}
                  </div>
                  <div class="text-[12px] text-muted mt-[1px]">
                    {{ t('account.lThemeHint') }}
                  </div>
                </div>
                <div class="grid grid-cols-3 gap-[10px] max-w-[440px]">
                  <button
                    type="button"
                    class="flex flex-col items-center gap-2 p-[14px_10px] rounded-[11px] border-[1.5px] cursor-pointer transition-colors"
                    :class="themePref === 'light' ? 'border-primary bg-primary/5' : 'border-default bg-default hover:border-primary/40'"
                    @click="setTheme('light')"
                  >
                    <UIcon
                      name="i-lucide-sun"
                      class="size-5"
                      :class="themePref === 'light' ? 'text-primary' : 'text-muted'"
                    />
                    <span
                      class="text-[12.5px] font-semibold"
                      :class="themePref === 'light' ? 'text-primary' : 'text-muted'"
                    >
                      {{ t('account.themeLight') }}
                    </span>
                  </button>
                  <button
                    type="button"
                    class="flex flex-col items-center gap-2 p-[14px_10px] rounded-[11px] border-[1.5px] cursor-pointer transition-colors"
                    :class="themePref === 'dark' ? 'border-primary bg-primary/5' : 'border-default bg-default hover:border-primary/40'"
                    @click="setTheme('dark')"
                  >
                    <UIcon
                      name="i-lucide-moon"
                      class="size-5"
                      :class="themePref === 'dark' ? 'text-primary' : 'text-muted'"
                    />
                    <span
                      class="text-[12.5px] font-semibold"
                      :class="themePref === 'dark' ? 'text-primary' : 'text-muted'"
                    >
                      {{ t('account.themeDark') }}
                    </span>
                  </button>
                  <button
                    type="button"
                    class="flex flex-col items-center gap-2 p-[14px_10px] rounded-[11px] border-[1.5px] cursor-pointer transition-colors"
                    :class="themePref === 'system' ? 'border-primary bg-primary/5' : 'border-default bg-default hover:border-primary/40'"
                    @click="setTheme('system')"
                  >
                    <UIcon
                      name="i-lucide-monitor"
                      class="size-5"
                      :class="themePref === 'system' ? 'text-primary' : 'text-muted'"
                    />
                    <span
                      class="text-[12.5px] font-semibold"
                      :class="themePref === 'system' ? 'text-primary' : 'text-muted'"
                    >
                      {{ t('account.themeSystem') }}
                    </span>
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Notifikasi card -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-[18px_20px]">
            <div class="text-[13px] font-semibold mb-1">
              {{ t('account.secNotifications') }}
            </div>
            <div class="text-[12px] text-dimmed mb-2">
              {{ t('account.secNotificationsHint') }}
            </div>
            <div>
              <div
                v-for="item in notifItems"
                :key="item.key"
                class="flex items-center justify-between gap-[14px] py-[13px] border-b border-default last:border-b-0"
              >
                <div class="flex items-center gap-[11px]">
                  <span
                    class="w-[34px] h-[34px] rounded-[9px] flex items-center justify-center flex-none"
                    :class="item.iconBg"
                  >
                    <UIcon
                      :name="item.icon"
                      class="size-[17px]"
                      :class="item.iconColor"
                    />
                  </span>
                  <div>
                    <div class="text-[13.5px] font-medium">
                      {{ item.label }}
                    </div>
                    <div class="text-[12px] text-muted">
                      {{ item.desc }}
                    </div>
                  </div>
                </div>
                <!-- Toggle switch -->
                <button
                  type="button"
                  role="switch"
                  :aria-checked="notif[item.key]"
                  :data-testid="`notif-${item.key}`"
                  class="relative w-[42px] h-[24px] rounded-full border-none cursor-pointer flex-none transition-colors"
                  :class="notif[item.key] ? 'bg-primary' : 'bg-muted'"
                  @click="toggleNotif(item.key)"
                >
                  <span
                    class="absolute top-[3px] w-[18px] h-[18px] rounded-full bg-white shadow-sm transition-all"
                    :class="notif[item.key] ? 'left-[21px]' : 'left-[3px]'"
                  />
                </button>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
