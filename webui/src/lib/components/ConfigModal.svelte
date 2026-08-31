<script>
  let {
    isOpen = false,
    configPath = '',
    fullConfig = null,
    loading = false,
    onClose,
    onSaveProfile,
    onDeleteProfile,
    onSaveSSHProxy,
    onDeleteSSHProxy,
    onSaveAI,
    onTestProfile,
    onTestSSHProxy,
    onTestAI
  } = $props();

  let activeTab = $state('profiles'); // 'profiles' | 'ssh_proxies' | 'ai'

  // Edit forms state
  let selectedProfileKey = $state('');
  let profileForm = $state({
    db: 'mysql',
    host: '127.0.0.1',
    port: 3306,
    user: 'root',
    password: '',
    database: '',
    description: '',
    unsafe_allow_write: false,
    allow_plaintext: true,
    ssh_proxy: ''
  });

  let selectedProxyKey = $state('');
  let proxyForm = $state({
    host: '',
    port: 22,
    user: '',
    identity_file: '',
    passphrase: '',
    known_hosts_file: '',
    skip_host_key: false
  });

  let aiForm = $state({
    provider: 'openai',
    base_url: 'https://api.openai.com/v1',
    api_key: '',
    model: 'gpt-4o',
    max_tokens: 8192,
    allow_plaintext: true
  });

  let isSubmitting = $state(false);
  let formError = $state('');

  // Testing states
  let isTesting = $state(false);
  let testResult = $state(null); // { ok: boolean, message: string, latency_ms?: number }

  function clearTestResult() {
    testResult = null;
  }

  function selectProfileToEdit(key) {
    clearTestResult();
    selectedProfileKey = key;
    if (key && fullConfig?.profiles?.[key]) {
      const p = fullConfig.profiles[key];
      profileForm = {
        db: p.db ?? p.DB ?? 'mysql',
        host: p.host ?? p.Host ?? '127.0.0.1',
        port: p.port ?? p.Port ?? ((p.db ?? p.DB) === 'pg' ? 5432 : 3306),
        user: p.user ?? p.User ?? 'root',
        password: p.password ?? p.Password ?? '',
        database: p.database ?? p.Database ?? '',
        description: p.description ?? p.Description ?? '',
        unsafe_allow_write: Boolean(p.unsafe_allow_write ?? p.UnsafeAllowWrite ?? false),
        allow_plaintext: p.allow_plaintext !== undefined ? Boolean(p.allow_plaintext) : (p.AllowPlaintext !== undefined ? Boolean(p.AllowPlaintext) : true),
        ssh_proxy: p.ssh_proxy ?? p.SSHProxy ?? ''
      };
    } else {
      selectedProfileKey = '';
      profileForm = {
        db: 'mysql',
        host: '127.0.0.1',
        port: 3306,
        user: 'root',
        password: '',
        database: '',
        description: '',
        unsafe_allow_write: false,
        allow_plaintext: true,
        ssh_proxy: ''
      };
    }
  }

  function selectProxyToEdit(key) {
    clearTestResult();
    selectedProxyKey = key;
    if (key && fullConfig?.ssh_proxies?.[key]) {
      const sp = fullConfig.ssh_proxies[key];
      proxyForm = {
        host: sp.host ?? sp.Host ?? '',
        port: sp.port ?? sp.Port ?? 22,
        user: sp.user ?? sp.User ?? '',
        identity_file: sp.identity_file ?? sp.IdentityFile ?? '',
        passphrase: sp.passphrase ?? sp.Passphrase ?? '',
        known_hosts_file: sp.known_hosts_file ?? sp.KnownHostsFile ?? '',
        skip_host_key: Boolean(sp.skip_host_key ?? sp.SkipHostKey ?? false)
      };
    } else {
      selectedProxyKey = '';
      proxyForm = {
        host: '',
        port: 22,
        user: '',
        identity_file: '',
        passphrase: '',
        known_hosts_file: '',
        skip_host_key: false
      };
    }
  }

  function syncAIForm() {
    clearTestResult();
    const ai = fullConfig?.ai;
    aiForm = {
      provider: ai?.provider || 'openai',
      base_url: ai?.base_url || 'https://api.openai.com/v1',
      api_key: ai?.api_key || '',
      model: ai?.model || 'gpt-4o',
      max_tokens: Number(ai?.max_tokens || 8192),
      allow_plaintext: ai?.allow_plaintext !== undefined ? Boolean(ai.allow_plaintext) : true
    };
  }

  $effect(() => {
    if (isOpen && fullConfig) {
      if (activeTab === 'profiles' && !selectedProfileKey) {
        const keys = Object.keys(fullConfig.profiles || {});
        if (keys.length > 0) {
          selectProfileToEdit(keys[0]);
        }
      } else if (activeTab === 'ssh_proxies' && !selectedProxyKey) {
        const keys = Object.keys(fullConfig.ssh_proxies || {});
        if (keys.length > 0) {
          selectProxyToEdit(keys[0]);
        }
      } else if (activeTab === 'ai') {
        syncAIForm();
      }
    }
  });

  // --- Profile Actions ---
  async function handleProfileSubmit(e) {
    e.preventDefault();
    const key = selectedProfileKey.trim();
    if (!key) {
      formError = 'Profile name is required';
      return;
    }
    isSubmitting = true;
    formError = '';
    try {
      await onSaveProfile?.(key, { ...profileForm, port: Number(profileForm.port) });
      selectProfileToEdit(key);
    } catch (err) {
      formError = err.message;
    } finally {
      isSubmitting = false;
    }
  }

  async function handleProfileDelete(key) {
    if (!key || !confirm(`Delete profile "${key}"?`)) return;
    isSubmitting = true;
    formError = '';
    try {
      await onDeleteProfile?.(key);
      const remaining = Object.keys(fullConfig?.profiles || {}).filter((k) => k !== key);
      selectProfileToEdit(remaining[0] || '');
    } catch (err) {
      formError = err.message;
    } finally {
      isSubmitting = false;
    }
  }

  async function handleTestProfile() {
    isTesting = true;
    testResult = null;
    try {
      const res = await onTestProfile?.(selectedProfileKey, { ...profileForm, port: Number(profileForm.port) });
      testResult = {
        ok: true,
        message: `Connection successful (${res.latency_ms ?? 0}ms)`
      };
    } catch (err) {
      testResult = {
        ok: false,
        message: err.message || 'Connection failed'
      };
    } finally {
      isTesting = false;
    }
  }

  // --- SSH Proxy Actions ---
  async function handleProxySubmit(e) {
    e.preventDefault();
    const key = selectedProxyKey.trim();
    if (!key) {
      formError = 'SSH Proxy name is required';
      return;
    }
    isSubmitting = true;
    formError = '';
    try {
      await onSaveSSHProxy?.(key, { ...proxyForm, port: Number(proxyForm.port) });
      selectProxyToEdit(key);
    } catch (err) {
      formError = err.message;
    } finally {
      isSubmitting = false;
    }
  }

  async function handleProxyDelete(key) {
    if (!key || !confirm(`Delete SSH Proxy "${key}"?`)) return;
    isSubmitting = true;
    formError = '';
    try {
      await onDeleteSSHProxy?.(key);
      const remaining = Object.keys(fullConfig?.ssh_proxies || {}).filter((k) => k !== key);
      selectProxyToEdit(remaining[0] || '');
    } catch (err) {
      formError = err.message;
    } finally {
      isSubmitting = false;
    }
  }

  async function handleTestSSHProxy() {
    isTesting = true;
    testResult = null;
    try {
      const res = await onTestSSHProxy?.(selectedProxyKey, { ...proxyForm, port: Number(proxyForm.port) });
      testResult = {
        ok: true,
        message: `SSH handshake successful (${res.latency_ms ?? 0}ms)`
      };
    } catch (err) {
      testResult = {
        ok: false,
        message: err.message || 'SSH connection failed'
      };
    } finally {
      isTesting = false;
    }
  }

  // --- AI Actions ---
  async function handleAISubmit(e) {
    e.preventDefault();
    isSubmitting = true;
    formError = '';
    try {
      await onSaveAI?.({ ...aiForm, max_tokens: Number(aiForm.max_tokens) });
    } catch (err) {
      formError = err.message;
    } finally {
      isSubmitting = false;
    }
  }

  async function handleTestAI() {
    isTesting = true;
    testResult = null;
    try {
      const res = await onTestAI?.({ ...aiForm, max_tokens: Number(aiForm.max_tokens) });
      testResult = {
        ok: true,
        message: `AI Model "${res.model || aiForm.model}" connected (${res.latency_ms ?? 0}ms)`
      };
    } catch (err) {
      testResult = {
        ok: false,
        message: err.message || 'AI service test failed'
      };
    } finally {
      isTesting = false;
    }
  }
</script>

{#if isOpen}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 overflow-y-auto animate-fade-in">
    <div
      role="button"
      tabindex="-1"
      aria-label="Close modal overlay"
      class="absolute inset-0"
      onclick={onClose}
      onkeydown={(e) => e.key === 'Escape' && onClose()}
    ></div>
    <div class="relative z-10 flex max-h-[90vh] w-full max-w-4xl flex-col rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg)] shadow-2xl overflow-hidden">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-[var(--panel-border)] px-5 py-3 bg-[var(--panel-inner)]">
        <div>
          <strong class="text-sm font-semibold text-[var(--text)]">Configuration & Profiles Manager</strong>
          {#if configPath}
            <p class="text-[11px] text-[var(--muted)] font-mono mt-0.5">{configPath}</p>
          {/if}
        </div>
        <button
          class="rounded-lg p-1 text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
          onclick={onClose}
        >
          ✕
        </button>
      </div>

      <!-- Tabs & Error -->
      <div class="flex items-center gap-2 border-b border-[var(--panel-border)] px-5 py-2">
        <button
          class={['xsql-tab', activeTab === 'profiles' && 'xsql-tab-active']}
          onclick={() => {
            activeTab = 'profiles';
            const keys = Object.keys(fullConfig?.profiles || {});
            if (!selectedProfileKey && keys.length > 0) selectProfileToEdit(keys[0]);
          }}
        >
          📂 Database Profiles ({Object.keys(fullConfig?.profiles || {}).length})
        </button>
        <button
          class={['xsql-tab', activeTab === 'ssh_proxies' && 'xsql-tab-active']}
          onclick={() => {
            activeTab = 'ssh_proxies';
            const keys = Object.keys(fullConfig?.ssh_proxies || {});
            if (!selectedProxyKey && keys.length > 0) selectProxyToEdit(keys[0]);
          }}
        >
          🔐 SSH Proxies ({Object.keys(fullConfig?.ssh_proxies || {}).length})
        </button>
        <button
          class={['xsql-tab', activeTab === 'ai' && 'xsql-tab-active']}
          onclick={() => {
            activeTab = 'ai';
            syncAIForm();
          }}
        >
          🤖 AI Settings
        </button>
      </div>

      {#if formError}
        <div class="bg-[var(--error-bg)] px-5 py-2 text-xs text-[var(--error-text)] border-b border-[var(--error-border)]">
          {formError}
        </div>
      {/if}

      <!-- Body -->
      <div class="xsql-scroll flex-1 overflow-y-auto p-5">
        {#if loading}
          <div class="py-12 text-center text-xs text-[var(--muted)]">Loading configuration…</div>
        {:else if activeTab === 'profiles'}
          <!-- Profiles Tab -->
          <div class="grid grid-cols-1 gap-6 md:grid-cols-[13rem_minmax(0,1fr)]">
            <!-- Left Profile List -->
            <div class="flex flex-col gap-2">
              <button
                class="xsql-button w-full justify-start border-dashed border-[var(--input-border)] text-xs text-[var(--accent)] hover:bg-[var(--accent-soft)]"
                onclick={() => selectProfileToEdit('')}
              >
                + New Profile
              </button>

              <div class="xsql-scroll flex max-h-80 flex-col gap-1 overflow-y-auto pr-1">
                {#each Object.keys(fullConfig?.profiles || {}) as pKey (pKey)}
                  {@const p = fullConfig.profiles[pKey]}
                  <button
                    class={[
                      'flex w-full items-center justify-between rounded-lg border px-2.5 py-1.5 text-left text-xs transition',
                      selectedProfileKey === pKey
                        ? 'border-[var(--accent-border)] bg-[var(--accent-soft)] font-semibold text-[var(--accent)]'
                        : 'border-transparent text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]'
                    ]}
                    onclick={() => selectProfileToEdit(pKey)}
                  >
                    <span class="truncate">{pKey}</span>
                    <span class="rounded bg-[var(--pill-bg)] px-1.5 py-0.2 text-[9px] uppercase font-mono text-[var(--pill-text)]">
                      {p.db ?? p.DB}
                    </span>
                  </button>
                {/each}
              </div>
            </div>

            <!-- Right Profile Form -->
            <form class="grid gap-3" onsubmit={handleProfileSubmit}>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pname">Profile Name</label>
                  <input
                    id="cfg-pname"
                    type="text"
                    required
                    class="xsql-input h-8"
                    placeholder="e.g. dev, production"
                    value={selectedProfileKey}
                    oninput={(e) => (selectedProfileKey = e.currentTarget.value)}
                  />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pdb">Database Type</label>
                  <select
                    id="cfg-pdb"
                    class="xsql-input h-8"
                    bind:value={profileForm.db}
                    onchange={() => {
                      if (profileForm.db === 'pg' && profileForm.port === 3306) profileForm.port = 5432;
                      if (profileForm.db === 'mysql' && profileForm.port === 5432) profileForm.port = 3306;
                    }}
                  >
                    <option value="mysql">MySQL</option>
                    <option value="pg">PostgreSQL</option>
                  </select>
                </div>
              </div>

              <div class="grid grid-cols-[minmax(0,1fr)_6rem] gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-phost">Host</label>
                  <input id="cfg-phost" type="text" class="xsql-input h-8 font-mono" bind:value={profileForm.host} placeholder="127.0.0.1" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pport">Port</label>
                  <input id="cfg-pport" type="number" class="xsql-input h-8 font-mono" bind:value={profileForm.port} />
                </div>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-puser">User</label>
                  <input id="cfg-puser" type="text" class="xsql-input h-8 font-mono" bind:value={profileForm.user} placeholder="root" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-ppass">Password / Secret</label>
                  <input id="cfg-ppass" type="password" class="xsql-input h-8 font-mono" bind:value={profileForm.password} placeholder="keyring:dev/pass or password" />
                </div>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pdatabase">Database Name</label>
                  <input id="cfg-pdatabase" type="text" class="xsql-input h-8 font-mono" bind:value={profileForm.database} placeholder="mydb" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pssh">SSH Proxy (Optional)</label>
                  <select id="cfg-pssh" class="xsql-input h-8" bind:value={profileForm.ssh_proxy}>
                    <option value="">None (Direct Connection)</option>
                    {#each Object.keys(fullConfig?.ssh_proxies || {}) as proxyKey}
                      <option value={proxyKey}>{proxyKey}</option>
                    {/each}
                  </select>
                </div>
              </div>

              <div>
                <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pdesc">Description</label>
                <input id="cfg-pdesc" type="text" class="xsql-input h-8" bind:value={profileForm.description} placeholder="e.g. Production Read-Only Replica" />
              </div>

              <div class="flex items-center gap-6 pt-1">
                <label class="flex items-center gap-2 text-xs text-[var(--text)] cursor-pointer">
                  <input type="checkbox" bind:checked={profileForm.unsafe_allow_write} class="rounded" />
                  <span>Allow Write Operations (Unsafe)</span>
                </label>
                <label class="flex items-center gap-2 text-xs text-[var(--text)] cursor-pointer">
                  <input type="checkbox" bind:checked={profileForm.allow_plaintext} class="rounded" />
                  <span>Allow Plaintext Password</span>
                </label>
              </div>

              <!-- Test Result Box -->
              {#if testResult}
                <div class={['rounded-lg p-2.5 text-xs font-mono border mt-1', testResult.ok ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20' : 'bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20']}>
                  {testResult.ok ? '✓ ' : '✕ '}{testResult.message}
                </div>
              {/if}

              <div class="mt-3 flex items-center justify-between gap-3 border-t border-[var(--panel-border)] pt-3">
                <button
                  type="button"
                  class="xsql-button text-xs"
                  disabled={isTesting || isSubmitting}
                  onclick={handleTestProfile}
                >
                  {#if isTesting}
                    <svg class="h-3 w-3 animate-spin text-[var(--accent)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" stroke-opacity="0.3"/><path d="M12 2a10 10 0 0 1 10 10"/>
                    </svg>
                    <span>Testing Connection…</span>
                  {:else}
                    <span>⚡ Test Connection</span>
                  {/if}
                </button>

                <div class="flex items-center gap-2">
                  {#if selectedProfileKey && fullConfig?.profiles?.[selectedProfileKey]}
                    <button
                      type="button"
                      class="xsql-button border-red-500/30 text-red-500 hover:bg-red-500/10 text-xs"
                      disabled={isSubmitting}
                      onclick={() => handleProfileDelete(selectedProfileKey)}
                    >
                      Delete Profile
                    </button>
                  {/if}
                  <button
                    type="submit"
                    class="xsql-button xsql-button-primary text-xs"
                    disabled={isSubmitting}
                  >
                    {isSubmitting ? 'Saving…' : 'Save Profile'}
                  </button>
                </div>
              </div>
            </form>
          </div>
        {:else if activeTab === 'ssh_proxies'}
          <!-- SSH Proxies Tab -->
          <div class="grid grid-cols-1 gap-6 md:grid-cols-[13rem_minmax(0,1fr)]">
            <!-- Left Proxy List -->
            <div class="flex flex-col gap-2">
              <button
                class="xsql-button w-full justify-start border-dashed border-[var(--input-border)] text-xs text-[var(--accent)] hover:bg-[var(--accent-soft)]"
                onclick={() => selectProxyToEdit('')}
              >
                + New SSH Proxy
              </button>

              <div class="xsql-scroll flex max-h-80 flex-col gap-1 overflow-y-auto pr-1">
                {#each Object.keys(fullConfig?.ssh_proxies || {}) as spKey (spKey)}
                  {@const sp = fullConfig.ssh_proxies[spKey]}
                  <button
                    class={[
                      'flex w-full items-center justify-between rounded-lg border px-2.5 py-1.5 text-left text-xs transition',
                      selectedProxyKey === spKey
                        ? 'border-[var(--accent-border)] bg-[var(--accent-soft)] font-semibold text-[var(--accent)]'
                        : 'border-transparent text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]'
                    ]}
                    onclick={() => selectProxyToEdit(spKey)}
                  >
                    <span class="truncate">{spKey}</span>
                    <span class="text-[10px] font-mono text-[var(--muted)]">{sp.host ?? sp.Host}</span>
                  </button>
                {/each}
              </div>
            </div>

            <!-- Right Proxy Form -->
            <form class="grid gap-3" onsubmit={handleProxySubmit}>
              <div>
                <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-spname">Proxy Name</label>
                <input
                  id="cfg-spname"
                  type="text"
                  required
                  class="xsql-input h-8"
                  placeholder="e.g. bastion, prod_ssh"
                  value={selectedProxyKey}
                  oninput={(e) => (selectedProxyKey = e.currentTarget.value)}
                />
              </div>

              <div class="grid grid-cols-[minmax(0,1fr)_6rem] gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-sphost">SSH Host</label>
                  <input id="cfg-sphost" type="text" class="xsql-input h-8 font-mono" bind:value={proxyForm.host} placeholder="bastion.example.com" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-spport">SSH Port</label>
                  <input id="cfg-spport" type="number" class="xsql-input h-8 font-mono" bind:value={proxyForm.port} />
                </div>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-spuser">SSH User</label>
                  <input id="cfg-spuser" type="text" class="xsql-input h-8 font-mono" bind:value={proxyForm.user} placeholder="admin" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-spkey">Identity File Path</label>
                  <input id="cfg-spkey" type="text" class="xsql-input h-8 font-mono" bind:value={proxyForm.identity_file} placeholder="~/.ssh/id_ed25519" />
                </div>
              </div>

              <div>
                <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-sppass">Passphrase (Optional)</label>
                <input id="cfg-sppass" type="password" class="xsql-input h-8 font-mono" bind:value={proxyForm.passphrase} placeholder="Keyring or passphrase" />
              </div>

              <div>
                <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-spknown">Known Hosts File (Optional)</label>
                <input id="cfg-spknown" type="text" class="xsql-input h-8 font-mono" bind:value={proxyForm.known_hosts_file} placeholder="~/.ssh/known_hosts" />
              </div>

              <div class="pt-1">
                <label class="flex items-center gap-2 text-xs text-[var(--text)] cursor-pointer">
                  <input type="checkbox" bind:checked={proxyForm.skip_host_key} class="rounded" />
                  <span>Skip Host Key Verification (Discouraged)</span>
                </label>
              </div>

              <!-- Test Result Box -->
              {#if testResult}
                <div class={['rounded-lg p-2.5 text-xs font-mono border mt-1', testResult.ok ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20' : 'bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20']}>
                  {testResult.ok ? '✓ ' : '✕ '}{testResult.message}
                </div>
              {/if}

              <div class="mt-3 flex items-center justify-between gap-3 border-t border-[var(--panel-border)] pt-3">
                <button
                  type="button"
                  class="xsql-button text-xs"
                  disabled={isTesting || isSubmitting}
                  onclick={handleTestSSHProxy}
                >
                  {#if isTesting}
                    <svg class="h-3 w-3 animate-spin text-[var(--accent)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" stroke-opacity="0.3"/><path d="M12 2a10 10 0 0 1 10 10"/>
                    </svg>
                    <span>Testing SSH…</span>
                  {:else}
                    <span>⚡ Test SSH Connection</span>
                  {/if}
                </button>

                <div class="flex items-center gap-2">
                  {#if selectedProxyKey && fullConfig?.ssh_proxies?.[selectedProxyKey]}
                    <button
                      type="button"
                      class="xsql-button border-red-500/30 text-red-500 hover:bg-red-500/10 text-xs"
                      disabled={isSubmitting}
                      onclick={() => handleProxyDelete(selectedProxyKey)}
                    >
                      Delete Proxy
                    </button>
                  {/if}
                  <button
                    type="submit"
                    class="xsql-button xsql-button-primary text-xs"
                    disabled={isSubmitting}
                  >
                    {isSubmitting ? 'Saving…' : 'Save Proxy'}
                  </button>
                </div>
              </div>
            </form>
          </div>
        {:else if activeTab === 'ai'}
          <!-- AI Settings Tab -->
          <div class="max-w-2xl mx-auto">
            <form class="grid gap-4" onsubmit={handleAISubmit}>
              <div class="rounded-lg bg-[var(--panel-inner)] p-3 border border-[var(--panel-border)] text-xs text-[var(--muted)] leading-relaxed">
                Configure LLM service provider for NL-to-SQL generation and AI database optimization in xsql.
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-aiprovider">Provider</label>
                  <select id="cfg-aiprovider" class="xsql-input h-8" bind:value={aiForm.provider}>
                    <option value="openai">OpenAI (or Compatible)</option>
                    <option value="deepseek">DeepSeek</option>
                    <option value="anthropic">Anthropic Claude</option>
                    <option value="ollama">Local Ollama</option>
                    <option value="custom">Custom Provider</option>
                  </select>
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-aimodel">Model Name</label>
                  <input
                    id="cfg-aimodel"
                    type="text"
                    required
                    class="xsql-input h-8 font-mono"
                    placeholder="e.g. gpt-4o, deepseek-chat, claude-3-5-sonnet"
                    bind:value={aiForm.model}
                  />
                </div>
              </div>

              <div>
                <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-aibase">API Base URL</label>
                <input
                  id="cfg-aibase"
                  type="text"
                  required
                  class="xsql-input h-8 font-mono"
                  placeholder="https://api.openai.com/v1"
                  bind:value={aiForm.base_url}
                />
              </div>

              <div class="grid grid-cols-[minmax(0,1fr)_8rem] gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-aikey">API Key</label>
                  <input
                    id="cfg-aikey"
                    type="password"
                    class="xsql-input h-8 font-mono"
                    placeholder="sk-..., or keyring:openai/api_key"
                    bind:value={aiForm.api_key}
                  />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-aitokens">Max Tokens</label>
                  <input
                    id="cfg-aitokens"
                    type="number"
                    class="xsql-input h-8 font-mono"
                    bind:value={aiForm.max_tokens}
                  />
                </div>
              </div>

              <div class="pt-1">
                <label class="flex items-center gap-2 text-xs text-[var(--text)] cursor-pointer">
                  <input type="checkbox" bind:checked={aiForm.allow_plaintext} class="rounded" />
                  <span>Allow Plaintext API Key in Config</span>
                </label>
              </div>

              <!-- Test Result Box -->
              {#if testResult}
                <div class={['rounded-lg p-2.5 text-xs font-mono border mt-1', testResult.ok ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20' : 'bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20']}>
                  {testResult.ok ? '✓ ' : '✕ '}{testResult.message}
                </div>
              {/if}

              <div class="mt-3 flex items-center justify-between gap-3 border-t border-[var(--panel-border)] pt-3">
                <button
                  type="button"
                  class="xsql-button text-xs"
                  disabled={isTesting || isSubmitting || !aiForm.api_key.trim()}
                  onclick={handleTestAI}
                >
                  {#if isTesting}
                    <svg class="h-3 w-3 animate-spin text-[var(--accent)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" stroke-opacity="0.3"/><path d="M12 2a10 10 0 0 1 10 10"/>
                    </svg>
                    <span>Testing AI API…</span>
                  {:else}
                    <span>⚡ Test AI Connection</span>
                  {/if}
                </button>

                <button
                  type="submit"
                  class="xsql-button xsql-button-primary text-xs"
                  disabled={isSubmitting}
                >
                  {isSubmitting ? 'Saving…' : 'Save AI Settings'}
                </button>
              </div>
            </form>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
