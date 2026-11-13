import { createShell } from '@myapplibs/web-platform/shell'
import { darowModule } from '@/darow/module'

createShell({
    title: 'darow',
    mark: 'D',
    owner: import.meta.env.VITE_OWNER,
    modules: [darowModule]
})
