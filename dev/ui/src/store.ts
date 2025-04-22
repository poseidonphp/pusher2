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

    const loadPusher = async (host: string|undefined, port: number, user_id: string = "") => {
        const options: EchoOptions<"pusher"> = {
            broadcaster: 'pusher',
            host: host,
            authEndpoint: 'http://localhost:8099/auth?user_id=' + user_id,
        }
        const pusherOptions: Options = {
            userAuthentication: {
                endpoint: 'http://localhost:8099/user-auth?user_id=' + user_id,
                transport: 'ajax',
                params:{'test': 'testing'}
            },
            wsHost: host,
            wsPort: host == undefined ? undefined : port,
            wssPort: host == undefined ? undefined : port,
            forceTLS: host == undefined,
            enabledTransports: ['ws', 'wss'],
            cluster: 'us2',
            authEndpoint: 'http://localhost:8099/auth?user_id=' + user_id,
            authTransport: 'ajax',
        }

        const pusherClient = new Pusher(import.meta.env.VITE_APP_KEY, pusherOptions)
        if (import.meta.env.VITE_ENABLE_USER_AUTHENTICATION == "true") {
        // if (import.meta.env.VITE_ENABLE_USER_AUTHENTICATION) {
            pusherClient.signin()
        }

        echo.value = new Echo({
            namespace: false,
            ...options,
            client: pusherClient
        })

        // pusher.value = new Pusher(import.meta.env.VITE_APP_KEY, pusherOptions )
    }

    return {count, pusher, loadPusher, echo}
})
