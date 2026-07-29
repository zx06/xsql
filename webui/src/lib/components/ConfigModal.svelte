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
    onDeleteSSHProxy
  } = $props();

  let activeTab = $state('profiles'); // 'profiles' | 'ssh_proxies'

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

  let isSubmitting = $state(false);
  let formError = $state('');

  function selectProfileToEdit(key) {
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

  // Auto select first item if none selected when config loads
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
      }
    }
  });

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
</script>

{#if isOpen}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 overflow-y-auto">
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
      <div class="flex items-center justify-between border-b border-[var(--panel-border)] px-5 py-4">
        <div>
          <strong class="text-base text-[var(--text)]">Configuration Manager (图形化配置中心)</strong>
          {#if configPath}
            <p class="text-xs text-[var(--muted)] font-mono mt-0.5">{configPath}</p>
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
      <div class="flex items-center gap-4 border-b border-[var(--panel-border)] px-5 py-2">
        <button
          class={['xsql-tab', activeTab === 'profiles' && 'xsql-tab-active']}
          onclick={() => {
            activeTab = 'profiles';
            const keys = Object.keys(fullConfig?.profiles || {});
            if (!selectedProfileKey && keys.length > 0) selectProfileToEdit(keys[0]);
          }}
        >
          Database Profiles ({Object.keys(fullConfig?.profiles || {}).length})
        </button>
        <button
          class={['xsql-tab', activeTab === 'ssh_proxies' && 'xsql-tab-active']}
          onclick={() => {
            activeTab = 'ssh_proxies';
            const keys = Object.keys(fullConfig?.ssh_proxies || {});
            if (!selectedProxyKey && keys.length > 0) selectProxyToEdit(keys[0]);
          }}
        >
          SSH Proxies ({Object.keys(fullConfig?.ssh_proxies || {}).length})
        </button>
      </div>

      {#if formError}
        <div class="bg-[var(--error-bg)] px-5 py-2 text-xs text-[var(--error-text)]">
          {formError}
        </div>
      {/if}

      <!-- Body -->
      <div class="xsql-scroll flex-1 overflow-y-auto p-5">
        {#if loading}
          <div class="py-12 text-center text-sm text-[var(--muted)]">Loading configuration…</div>
        {:else if activeTab === 'profiles'}
          <!-- Profiles Tab -->
          <div class="grid grid-cols-1 gap-6 md:grid-cols-[14rem_minmax(0,1fr)]">
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
                      'flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-xs transition',
                      selectedProfileKey === pKey
                        ? 'border-[var(--accent-border)] bg-[var(--accent-soft)] font-medium text-[var(--text)]'
                        : 'border-transparent text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]'
                    ]}
                    onclick={() => selectProfileToEdit(pKey)}
                  >
                    <span class="truncate">{pKey}</span>
                    <span class="rounded bg-[var(--pill-bg)] px-1.5 py-0.5 text-[10px] uppercase text-[var(--pill-text)]">
                      {p.db ?? p.DB}
                    </span>
                  </button>
                {/each}
              </div>
            </div>

            <!-- Right Profile Form -->
            <form class="grid gap-3.5" onsubmit={handleProfileSubmit}>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pname">Profile Name</label>
                  <input
                    id="cfg-pname"
                    type="text"
                    required
                    class="xsql-input"
                    placeholder="e.g. dev, production"
                    value={selectedProfileKey}
                    oninput={(e) => (selectedProfileKey = e.currentTarget.value)}
                  />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pdb">Database Type</label>
                  <select
                    id="cfg-pdb"
                    class="xsql-input"
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
                  <input id="cfg-phost" type="text" class="xsql-input" bind:value={profileForm.host} placeholder="127.0.0.1" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pport">Port</label>
                  <input id="cfg-pport" type="number" class="xsql-input" bind:value={profileForm.port} />
                </div>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-puser">User</label>
                  <input id="cfg-puser" type="text" class="xsql-input" bind:value={profileForm.user} placeholder="root" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-ppass">Password / Keyring</label>
                  <input id="cfg-ppass" type="password" class="xsql-input" bind:value={profileForm.password} placeholder="keyring:dev/pass or secret" />
                </div>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pdatabase">Database Name</label>
                  <input id="cfg-pdatabase" type="text" class="xsql-input" bind:value={profileForm.database} placeholder="mydb" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pssh">SSH Proxy (Optional)</label>
                  <select id="cfg-pssh" class="xsql-input" bind:value={profileForm.ssh_proxy}>
                    <option value="">None (Direct Connection)</option>
                    {#each Object.keys(fullConfig?.ssh_proxies || {}) as proxyKey}
                      <option value={proxyKey}>{proxyKey}</option>
                    {/each}
                  </select>
                </div>
              </div>

              <div>
                <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-pdesc">Description</label>
                <input id="cfg-pdesc" type="text" class="xsql-input" bind:value={profileForm.description} placeholder="e.g. Primary MySQL DB" />
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

              <div class="mt-3 flex items-center justify-end gap-3 border-t border-[var(--panel-border)] pt-3">
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
            </form>
          </div>
        {:else if activeTab === 'ssh_proxies'}
          <!-- SSH Proxies Tab -->
          <div class="grid grid-cols-1 gap-6 md:grid-cols-[14rem_minmax(0,1fr)]">
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
                      'flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-xs transition',
                      selectedProxyKey === spKey
                        ? 'border-[var(--accent-border)] bg-[var(--accent-soft)] font-medium text-[var(--text)]'
                        : 'border-transparent text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]'
                    ]}
                    onclick={() => selectProxyToEdit(spKey)}
                  >
                    <span class="truncate">{spKey}</span>
                    <span class="text-[10px] text-[var(--muted)]">{sp.host ?? sp.Host}</span>
                  </button>
                {/each}
              </div>
            </div>

            <!-- Right Proxy Form -->
            <form class="grid gap-3.5" onsubmit={handleProxySubmit}>
              <div>
                <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-spname">Proxy Name</label>
                <input
                  id="cfg-spname"
                  type="text"
                  required
                  class="xsql-input"
                  placeholder="e.g. bastion, prod_ssh"
                  value={selectedProxyKey}
                  oninput={(e) => (selectedProxyKey = e.currentTarget.value)}
                />
              </div>

              <div class="grid grid-cols-[minmax(0,1fr)_6rem] gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-sphost">SSH Host</label>
                  <input id="cfg-sphost" type="text" class="xsql-input" bind:value={proxyForm.host} placeholder="bastion.example.com" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-spport">SSH Port</label>
                  <input id="cfg-spport" type="number" class="xsql-input" bind:value={proxyForm.port} />
                </div>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-spuser">SSH User</label>
                  <input id="cfg-spuser" type="text" class="xsql-input" bind:value={proxyForm.user} placeholder="admin" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-spkey">Identity File Path</label>
                  <input id="cfg-spkey" type="text" class="xsql-input" bind:value={proxyForm.identity_file} placeholder="~/.ssh/id_ed25519" />
                </div>
              </div>

              <div>
                <label class="block text-xs font-medium text-[var(--muted)] mb-1" for="cfg-sppass">Passphrase (Optional)</label>
                <input id="cfg-sppass" type="password" class="xsql-input" bind:value={proxyForm.passphrase} placeholder="Keyring or passphrase" />
              </div>

              <div class="mt-3 flex items-center justify-end gap-3 border-t border-[var(--panel-border)] pt-3">
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
            </form>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
