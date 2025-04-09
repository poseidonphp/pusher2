import { defineStore} from "pinia";
import {ref} from "vue";
import Pusher from 'pusher-js/with-encryption'
import type {Options} from 'pusher-js/with-encryption'
import Echo from 'laravel-echo'
import type {EchoOptions} from 'laravel-echo'


export const useAppStore = defineStore('app', () => {
    const count = ref(0)
    const pusher = ref<Pusher | null>(null)
    const echo = ref<Echo<"pusher"> | null>(null)

    const loadPusher = async (host: string, port: number, user_id: string = "") => {

        const options: EchoOptions<"pusher"> = {
            broadcaster: 'pusher',
            host: host,
            authEndpoint: 'http://localhost:8099/auth?user_id=' + user_id,
        }
        const pusherOptions: Options = {
            wsHost: host,
            wsPort: port,
            wssPort: port,
            forceTLS: false,
            enabledTransports: ['ws', 'wss'],
            cluster: 'mt1',
            authEndpoint: 'http://localhost:8099/auth?user_id=' + user_id,
            authTransport: 'ajax',
        }
        echo.value = new Echo({
            namespace: false,
            ...options,
            client: new Pusher(import.meta.env.VITE_APP_KEY, pusherOptions)
        })
        // pusher.value = new Pusher(import.meta.env.VITE_APP_KEY, pusherOptions )
    }

    return {count, pusher, loadPusher, echo}
})
