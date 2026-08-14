<script>
  let {
    profiles = [],
    selectedProfile = '',
    selectedProfileMeta = null,
    themeMode = 'auto',
    authRequired = false,
    authToken = '',
    tableCount = 0,
    schemaLoading = false,
    onProfileChange,
    onThemeChange,
    onOpenConfig,
    onOpenShortcuts
  } = $props();

  function cycleTheme() {
    if (themeMode === 'auto') onThemeChange?.('black');
    else if (themeMode === 'black') onThemeChange?.('white');
    else onThemeChange?.('auto');
  }
</script>

<header class="xsql-panel mb-2 flex h-11 shrink-0 items-center justify-between gap-3 px-3 py-1 text-xs">
  <!-- Left branding & Profile Picker -->
  <div class="flex items-center gap-3 min-w-0">
    <div class="flex items-center gap-2 shrink-0">
      <div class="flex h-6 w-6 items-center justify-center rounded-lg bg-[var(--accent)] text-white shadow-xs">
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <ellipse cx="12" cy="5" rx="9" ry="3"/>
          <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
          <path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/>
        </svg>
      </div>
      <span class="font-bold tracking-tight text-[var(--text)] text-sm">xsql</span>
      <span class="hidden sm:inline-block rounded bg-[var(--pill-bg)] px-1.5 py-0.5 text-[9px] font-semibold text-[var(--pill-text)] uppercase font-mono">
        AI Console
      </span>
    </div>

    <div class="h-4 w-px bg-[var(--panel-border)] shrink-0"></div>

    <!-- Profile Selector Dropdown -->
    <div class="flex items-center gap-1.5 min-w-0">
      <label for="navbar-profile-select" class="text-[var(--muted)] shrink-0 text-[11px] font-medium hidden md:inline">
        Profile:
      </label>
      <div class="relative flex items-center min-w-0">
        <select
          id="navbar-profile-select"
          class="h-7 rounded-lg border border-[var(--input-border)] bg-[var(--input-bg)] pl-2.5 pr-7 text-xs font-medium text-[var(--text)] outline-none transition hover:border-[var(--accent-border)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)] cursor-pointer"
          value={selectedProfile}
          onchange={(e) => onProfileChange?.(e.currentTarget.value)}
        >
          {#if profiles.length === 0}
            <option value="" disabled>No profiles found</option>
          {/if}
          {#each profiles as profile (profile.name)}
            <option value={profile.name}>
              {profile.name} ({profile.db})
            </option>
          {/each}
        </select>
      </div>

      {#if selectedProfileMeta}
        <span class="inline-flex shrink-0 items-center rounded bg-[var(--tag-bg)] px-1.5 py-0.5 text-[10px] uppercase font-mono font-bold text-[var(--tag-text)]">
          {selectedProfileMeta.db}
        </span>
      {/if}

      <!-- Safe Readonly Tag -->
      <span
        class="hidden lg:inline-flex shrink-0 items-center gap-1 rounded bg-emerald-500/10 px-2 py-0.5 text-[10px] font-medium text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
        title="Double-layer Read-Only safety active (Static Analysis + Transaction RO)"
      >
        <svg class="h-2.5 w-2.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <rect width="18" height="11" x="3" y="11" rx="2" ry="2"/>
          <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
        </svg>
        Read-Only
      </span>
    </div>
  </div>

  <!-- Center DB Health / Objects Info -->
  <div class="hidden md:flex items-center gap-2 text-[11px] text-[var(--muted)]">
    {#if schemaLoading}
      <span class="inline-block h-2 w-2 animate-pulse rounded-full bg-amber-400"></span>
      <span>Syncing metadata…</span>
    {:else if selectedProfile}
      <span class="inline-block h-2 w-2 rounded-full bg-emerald-400"></span>
      <span>{tableCount} tables ready</span>
    {/if}
  </div>

  <!-- Right Actions -->
  <div class="flex items-center gap-1.5 shrink-0">
    <button
      class="xsql-button px-2 py-1 text-xs text-[var(--muted)] hover:text-[var(--text)]"
      onclick={onOpenShortcuts}
      title="View Keyboard Shortcuts"
    >
      <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect width="18" height="12" x="3" y="6" rx="2"/><path d="M7 10h.01M12 10h.01M17 10h.01M7 14h.01M12 14h4"/>
      </svg>
      <span class="hidden sm:inline">Shortcuts</span>
    </button>

    <button
      class="xsql-button px-2 py-1 text-xs text-[var(--muted)] hover:text-[var(--text)]"
      onclick={onOpenConfig}
      title="Manage DB Profiles & SSH Proxies"
    >
      <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.38a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
        <circle cx="12" cy="12" r="3"/>
      </svg>
      <span class="hidden sm:inline">Config</span>
    </button>

    <!-- Theme Switcher Button -->
    <button
      class="flex h-7 w-7 items-center justify-center rounded-lg border border-[var(--panel-border)] bg-[var(--panel-inner)] text-[var(--muted)] transition hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
      onclick={cycleTheme}
      title={`Theme: ${themeMode} (click to toggle)`}
    >
      {#if themeMode === 'black'}
        <!-- Moon icon -->
        <svg class="h-3.5 w-3.5 text-blue-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>
        </svg>
      {:else if themeMode === 'white'}
        <!-- Sun icon -->
        <svg class="h-3.5 w-3.5 text-amber-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/>
        </svg>
      {:else}
        <!-- Auto / Monitor icon -->
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect width="20" height="14" x="2" y="3" rx="2"/><line x1="8" x2="16" y1="21" y2="21"/><line x1="12" x2="12" y1="17" y2="21"/>
        </svg>
      {/if}
    </button>
  </div>
</header>
