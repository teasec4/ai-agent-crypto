<script lang="ts">
  const serverPresets = [
    'http://127.0.0.1:8080',
    'http://192.168.31.89:8080'
  ];

  let serverUrl = $state(serverPresets[0]);
  let message = $state('');
</script>

<main class="shell">
  <aside class="sidebar">
    <div class="brand">
      <span class="brand-mark">AI</span>
      <div>
        <h1>Agent Client</h1>
        <p>Electron + Svelte 5</p>
      </div>
    </div>

    <section class="panel">
      <h2>Server</h2>
      <div class="preset-list" aria-label="Server presets">
        {#each serverPresets as preset}
          <button
            class:active={serverUrl === preset}
            type="button"
            onclick={() => (serverUrl = preset)}
          >
            {preset.replace('http://', '')}
          </button>
        {/each}
      </div>
      <label>
        <span>API URL</span>
        <input bind:value={serverUrl} />
      </label>
    </section>
  </aside>

  <section class="workspace">
    <header class="topbar">
      <div>
        <p class="eyebrow">Desktop workspace</p>
        <h2>Ready for the agent UI</h2>
      </div>
      <button type="button">Connect</button>
    </header>

    <div class="chat-surface">
      <div class="message assistant">
        <span>assistant</span>
        <p>Svelte 5 renderer is mounted. Next step is wiring this to the existing server API.</p>
      </div>
      <div class="message user">
        <span>you</span>
        <p>Use this shell as the Electron client foundation.</p>
      </div>
    </div>

    <form
      class="composer"
      onsubmit={(event) => {
        event.preventDefault();
        message = '';
      }}
    >
      <input bind:value={message} placeholder="Message the agent..." />
      <button disabled={!message.trim()} type="submit">Send</button>
    </form>
  </section>
</main>
