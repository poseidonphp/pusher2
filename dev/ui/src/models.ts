import type { PresenceChannel, Channel } from "laravel-echo";
import Echo from "laravel-echo";

// Extract the return type from actual Echo methods
export type PusherPrivateChannel = ReturnType<Echo['private']>;
export type PusherPresenceChannel = ReturnType<Echo['join']>;
export type PusherEncryptedPrivateChannel = ReturnType<Echo['encryptedPrivate']>;
export type PusherChannel = ReturnType<Echo['channel']>;


export type EchoUser = {
    id: string
    name: string
    email: string
}

export type JoinedChannel = {
    name: string
    messages: string[]
    isPublic: boolean
    obj?:  PusherPrivateChannel | PusherEncryptedPrivateChannel | PusherPresenceChannel | PusherChannel
    // obj?:  PusherEncryptedPrivateChannel<"pusher"> | PusherPrivateChannel<"pusher"> | PusherPresenceChannel<"pusher"> | PusherChannel<"pusher">
}