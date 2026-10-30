<template>
  <div class="tw-flex tw-flex-col tw-gap-5">
    <div class="tw-text-xl tw-font-medium tw-text-dark-green sm:tw-text-2xl">
      Assistant tokens
    </div>
    <div class="tw-text-black">
      Create a token and paste it into Claude or Cursor as
      <span class="tw-font-medium">Authorization: Bearer</span>. The token is
      shown once. Revoke it here to disconnect that assistant.
    </div>
    <div class="tw-flex tw-flex-col tw-gap-3 sm:tw-flex-row">
      <input
        v-model="label"
        maxlength="40"
        placeholder="Label, for example Laptop"
        class="tw-flex-1 tw-border tw-border-light-gray-stroke tw-px-3 tw-py-2"
      />
      <button
        type="button"
        class="tw-bg-dark-green tw-px-4 tw-py-2 tw-text-white"
        @click="createToken"
      >
        Create token
      </button>
    </div>
    <div v-if="plaintext" class="tw-border tw-border-light-gray-stroke tw-p-4">
      <div class="tw-mb-2 tw-text-sm tw-text-dark-gray">
        Copy this token now. Muster will not show it again.
      </div>
      <div class="tw-break-all tw-font-mono tw-text-sm">{{ plaintext }}</div>
    </div>
    <div v-if="error" class="tw-text-red">{{ error }}</div>
    <div v-if="tokens.length === 0" class="tw-text-dark-gray">No tokens yet.</div>
    <div v-for="token in tokens" :key="token.id" class="tw-flex tw-flex-col tw-gap-1 tw-border-b tw-border-light-gray-stroke tw-py-3 sm:tw-flex-row sm:tw-items-center sm:tw-justify-between">
      <div>
        <div class="tw-font-medium">{{ token.label }}</div>
        <div class="tw-text-sm tw-text-dark-gray">
          Expires {{ token.expiresAt }}
          <span v-if="token.revokedAt"> · Revoked</span>
        </div>
      </div>
      <button
        v-if="!token.revokedAt"
        type="button"
        class="tw-text-red"
        @click="revoke(token.id)"
      >
        Revoke
      </button>
    </div>
  </div>
</template>

<script>
import { get, post, _delete } from "@/utils"

export default {
  name: "McpTokens",
  data: () => ({
    label: "",
    tokens: [],
    plaintext: "",
    error: "",
  }),
  created() {
    this.load()
  },
  methods: {
    load() {
      get("/user/mcp-tokens")
        .then((rows) => {
          this.tokens = rows || []
        })
        .catch(() => {
          this.error = "Could not load tokens."
        })
    },
    createToken() {
      this.error = ""
      this.plaintext = ""
      post("/user/mcp-tokens", { label: this.label })
        .then((row) => {
          this.plaintext = row.token
          this.label = ""
          this.load()
        })
        .catch((err) => {
          const parsed = err && err.parsed
          this.error = (parsed && parsed.error) || "Could not create a token."
        })
    },
    revoke(id) {
      this.error = ""
      _delete("/user/mcp-tokens/" + id)
        .then(() => {
          this.load()
        })
        .catch(() => {
          this.error = "Could not revoke that token."
        })
    },
  },
}
</script>
