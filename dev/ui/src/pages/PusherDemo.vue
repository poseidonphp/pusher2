<template>
  <div class="container mx-auto px-4 py-8">
    <!-- Optional Header -->
    <header class="mb-6">
      <h1 class="text-2xl font-bold text-crimson-500 text-shadow-lg drop-shadow-[0_1.2px_1.2px_rgba(0,0,0,0.5)]">
        Pusher Dev UI
        <span v-if="connectionStatus == 'connected'" title="connected">🟢</span>
        <span v-else-if="connectionStatus == 'connecting'" title="connecting">🟡</span>
        <span v-else-if="connectionStatus == 'disconnected'" title="disconnected">🟠</span>
        <span v-else-if="connectionStatus == 'unavailable'" title="unavailable">🔴</span>
        <span v-else :title="connectionStatus">⚫️</span>
      </h1>
      <h4>Using AppKey: <span class="dark:text-crimson-300 dark:bg-crimson-900 text-crimson-800 bg-crimson-300 px-2 py-1 rounded-lg">{{ appKey }}</span></h4>
    </header>

    <!-- 3-Column Layout -->
    <div class="grid grid-cols-1 md:grid-cols-7 grid-flow-col gap-6">
      <!-- Column 1 -->
      <div class="p-6 rounded-lg shadow border-2 border-crimson-500 col-span-2 ">
        <h2 class="text-xl font-semibold mb-4 text-shadow-lg ">Presence Channel Members</h2>
        <div>
          <ul>
            <li v-for="user in presenceUsers" :key="user.id">
              {{ user.name }}
            </li>
          </ul>
        </div>
      </div>

      <!-- Column 2 -->
      <div class="col-span-2 ">
        <div class="grid grid-rows-3 gap-6">
          <div class="p-6 rounded-lg shadow border-2 border-crimson-500 row-span-2">
            <h2 class="text-xl font-semibold mb-4 text-shadow-lg ">Join a new channel</h2>
            <form>
              <div>
                <input v-model="channelToJoin" placeholder="Channel to join" class="border p-2 rounded mb-1" />

                <div class="text-sm text-crimson-300 mb-4">{{ computedChannelToJoin }}</div>
                <label>
                  <input type="radio" id="private" value="private" v-model="channelToJoinType" /> Private
                </label>
                &nbsp; | &nbsp;
                <label>
                  <input type="radio" id="private-encrypted" value="encrypted" v-model="channelToJoinType" /> Encrypted
                </label>
                &nbsp; | &nbsp;
                <label>
                  <input type="radio" id="public" value="public" v-model="channelToJoinType" /> Public
                </label>


                <br />
                <button type="submit" @click.prevent="joinNewChannel(channelToJoin)" class="button mt-4">
                  JOIN
                </button>

              </div>
            </form>
          </div>

          <div class="p-6 rounded-lg shadow border-2 border-crimson-500  ">
            <h2 class="text-xl font-semibold">Whisper</h2>
            <span class="text-xs text-crimson-400 text-shadow-lg ">Whisper on {{ selectedChannel }}</span>
            <br />
            <form>
              <input type="text" v-model="whisperMessage" placeholder="Whisper message" class="border p-2 rounded mb-4" />
              <button :disabled="!selectedChannel" @click.prevent="sendWhisper" class="button">Send</button>
            </form>
          </div>
        </div>
      </div>


      <!-- Column 3 -->
      <div class="p-6 rounded-lg shadow border-2 border-crimson-500 col-span-3">
        <h2 class="text-xl font-semibold mb-4 text-shadow-lg ">Joined Channels</h2>
        <div v-for="channel in joinedChannels" :key="channel.name" class="p-3 border border-gray-300 rounded-lg mb-4">
          <h3 class="text-lg font-semibold text-left">
            <label>
              <input type="radio" name="selectedChannel" :value="channel.name" v-model="selectedChannel" />
              {{ channel.name }}
              <button @click="triggerEvent(channel.name)" class="px-3 border border-crimson-500 rounded dark:bg-gray-900 text-xl"
              title="Trigger event on this channel"
              >📡</button>
              &nbsp;
              &nbsp;
              <button title="Close Connection" @click="disconnect(channel.name)" class="px-3 border border-crimson-500 rounded dark:bg-gray-900 text-xl">❌</button>
            </label>

          </h3>
          <ul class="pl-4 text-left">
            <li v-for="(message, id) in channel.messages" :key="id">
              <span v-html="message"></span>
            </li>
          </ul>
        </div>
      </div>
    </div>

    <!-- Optional Footer -->
    <footer class="mt-6 text-center text-gray-500">
      <p>This page is just used for testing connectivity to the pusher server.
        <br />
        It uses a mock auth server that allows all connections, and will generate a username on the fly using the current minute of the hour.</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import {computed, onMounted, ref, watch} from "vue";
import {useAppStore} from "../store.ts";
import axios from "axios";
import type {EchoUser, JoinedChannel, PusherChannel} from "../models.ts";
import {storeToRefs} from "pinia";


const store = useAppStore()

const presenceUsers = ref<EchoUser[]>([])
const joinedChannels = ref<JoinedChannel[]>([])
const channelToJoin = ref<string>("")
const channelToJoinType = ref<string>("private")

const appKey = import.meta.env.VITE_APP_KEY || "<NOT SET> Be sure to set this in .env file"

const selectedChannel = ref("")
const whisperMessage = ref("")
const userId = ref("")

const connectionStatus = ref("disconnected")

const {echo} = storeToRefs(store)

// const connectionStatus = computed(() => {
//   if (store.echo) {
//     return store.echo.connector.pusher.connection.state
//   } else {
//     return "disconnected"
//   }
// })


const computedChannelToJoin = computed(() => {
  if (channelToJoinType.value === "private") {
    return `private-${channelToJoin.value}`
  } else if (channelToJoinType.value === "encrypted") {
    return `private-encrypted-${channelToJoin.value}`
  } else {
    return channelToJoin.value
  }
})

const sendWhisper = () => {
  if (store.echo && selectedChannel.value) {
    const channel = joinedChannels.value.find(c => c.name === selectedChannel.value)
    if (channel && channel.obj) {
        channel.obj.whisper("testEvent", whisperMessage.value)
    }
  }
}

const joinPresenceChannel = () => {
  if (store.echo) {
    const presenceChannel = store.echo.join("users")
    presenceChannel.here((users: EchoUser[]) => {
      console.log("Users present", users)
      presenceUsers.value = users
    })
    presenceChannel.joining((user:EchoUser) => {
      console.log("User joining", user)
      presenceUsers.value.push(user)
    })
    presenceChannel.leaving((user: EchoUser) => {
      console.log("User leaving", user)
      presenceUsers.value = presenceUsers.value.filter(u => u.id !== user.id)
    })
    presenceChannel.listenToAll((evt: string, data: any) => {
      console.log("Event received: ", evt, data)
      addMessageToChannel("presence-users", evt, data)
    })
    joinedChannels.value.push({
      name: "presence-users",
      messages: [],
      isPublic: false,
      obj: presenceChannel
    })
  }
}

const triggerEvent = (channel: string)  => {
  if (store.echo && channel.length > 0) {
    const channelToTrigger = joinedChannels.value.find(c => c.name === channel)
    if (!channelToTrigger) {
      return
    }
    axios.post("http://localhost:8099/test/" + channelToTrigger.name + "?name=testEvent", {
      data: "Test data from manual trigger"
    })
  }
}

const disconnect = (channel: string) => {
  if (store.echo && channel.length > 0) {
    const channelToLeave = joinedChannels.value.find(c => c.name === channel)
    if (channelToLeave) {
      store.echo.leave(channelToLeave.name)
      joinedChannels.value = joinedChannels.value.filter(c => c.name !== channel)
    }
  }
}

const joinNewChannel = (channel: string) => {
  if (store.echo && channel.length > 0) {
    let channelName = channel
    if (channelToJoinType.value === "encrypted") {
      channelName = `private-encrypted-${channel}`

    } else if (channelToJoinType.value === "private") {
      channelName = `private-${channel}`
    }

    const newJoinedChannel = ref<JoinedChannel>({
      name: channelName,
      messages: [],
      isPublic: false
    })
    let newChannel: PusherChannel;

    if(channelToJoinType.value === "encrypted") {
      newChannel = store.echo.encryptedPrivate(channel)
    } else if (channelToJoinType.value === "private") {
      newChannel = store.echo.private(channel)
    } else {
      newChannel = store.echo.channel(channel)
      newJoinedChannel.value.isPublic = true
    }
    newChannel.listenToAll((evt: string, data: any) => {
      console.log("Event received: ", evt, data)
      addMessageToChannel(channelName, evt, data)
    })
    newJoinedChannel.value.obj = newChannel

    joinedChannels.value.push(newJoinedChannel.value)
  }
}

const addMessageToChannel = (channel: string, evt: string, message: string) => {
  const channelToUpdate = joinedChannels.value.find(c => c.name === channel)
  if (channelToUpdate) {
    if (evt.startsWith('.')) {
      evt = evt.substring(1)
    }
    let timeString = new Date().toLocaleTimeString([], {hour: '2-digit', minute:'2-digit', second:'2-digit', hour12: false})
    timeString = `<span class="text-xs text-crimson-400">${timeString}</span>`

    if (evt.startsWith('client-')) {
      evt = evt.substring(7)
      evt = `<span class="text-sm text-sky-400"><i>whisper</i></span><span class="text-sky-400 font-bold"> ${evt}</span>`

    } else {
      evt = `<span class="text-crimson-500 font-bold">${evt}</span>`
    }

    channelToUpdate.messages.unshift(`${timeString} ${evt}: ${message}`)
    if(channelToUpdate.messages.length > 5) {
      channelToUpdate.messages.pop()
    }
  }
}

const checkConnectionStatus = () => {
      if (store.echo) {
        connectionStatus.value = store.echo.connector.pusher.connection.state
      } else {
        connectionStatus.value = "unavailable"
      }
}


onMounted(async () => {
  // get query parameter for "host" and "port"
  const urlParams = new URLSearchParams(window.location.search);

  const host = urlParams.get("host") || "localhost"
  const port = urlParams.get("port") || "6001"
  let uid = urlParams.get("user_id") || ""
  let connectUserChannels = true
  if (uid == "") {
    connectUserChannels = false
    uid = Math.floor(Math.random() * 1000).toString()
  }
  userId.value = uid
  setInterval(checkConnectionStatus, 500)

  // Connect to Pusher
  await store.loadPusher(host, parseInt(port), uid)
  joinPresenceChannel()

  if(connectUserChannels) {
    channelToJoin.value = "user-" + uid
    channelToJoinType.value = "private"
    joinNewChannel(channelToJoin.value)
    joinNewChannel("all-logged-in-users")
    channelToJoinType.value = "encrypted"
    joinNewChannel("user-" + uid)
    channelToJoinType.value = "public"
    joinNewChannel("all-public-users")
    channelToJoin.value = ""
  }

})

</script>