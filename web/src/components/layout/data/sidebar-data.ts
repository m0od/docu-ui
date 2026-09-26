import { FileKey, FolderOpen, Palette, Rocket, Settings } from 'lucide-react'
import { type SidebarData } from '../types'

export const sidebarData: SidebarData = {
  navGroups: [
    {
      title: 'General',
      items: [
        {
          title: 'Env files',
          url: '/env-files',
          icon: FileKey,
        },
      ],
    },
    {
      title: 'Other',
      items: [
        {
          title: 'Settings',
          icon: Settings,
          items: [
            {
              title: 'Env folder',
              url: '/settings/env-folder',
              icon: FolderOpen,
            },
            {
              title: 'Apply',
              url: '/settings/apply',
              icon: Rocket,
            },
            {
              title: 'Appearance',
              url: '/settings/appearance',
              icon: Palette,
            },
          ],
        },
      ],
    },
  ],
}
