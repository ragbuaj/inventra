import { createStore } from './helpers'

export interface Notification {
  id: string
  icon: string
  iconBg: string
  iconColor: string
  title: string
  time: string
  read: boolean
}

const notificationSeed: Notification[] = [
  {
    id: 'n-1',
    icon: 'i-lucide-check-square',
    iconBg: 'bg-primary/10',
    iconColor: 'text-primary',
    title: 'notifications.item.approvalPending',
    time: 'notifications.time.fiveMinutesAgo',
    read: false
  },
  {
    id: 'n-2',
    icon: 'i-lucide-wrench',
    iconBg: 'bg-warning/15',
    iconColor: 'text-warning',
    title: 'notifications.item.maintenanceDue',
    time: 'notifications.time.oneHourAgo',
    read: false
  },
  {
    id: 'n-3',
    icon: 'i-lucide-package',
    iconBg: 'bg-muted',
    iconColor: 'text-muted',
    title: 'notifications.item.assetReturned',
    time: 'notifications.time.threeHoursAgo',
    read: true
  }
]

export const notificationStore = createStore<Notification>(notificationSeed)
