import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { router } from './router'
import { initOidc, restoreBrowserSession } from './auth/oidc'

// The manager is built and the boot recovery run before the app ever
// mounts, so the first frame already knows whether anyone is signed in and
// no control flickers from enabled to disabled. A failed /auth/config
// (inside initOidc) leaves the app usable with no login control rather than
// blocking the mount.
async function bootstrap() {
    await initOidc()
    await restoreBrowserSession()
    createApp(App).use(router).mount('#app')
}

void bootstrap()
