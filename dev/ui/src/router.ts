import { createMemoryHistory, createRouter } from 'vue-router'

import HomeView from './pages/PusherDemo.vue'

const routes = [
    { path: '/', component: HomeView, name: 'home' },
]

const router = createRouter({
    history: createMemoryHistory(),
    routes,
})

export default router