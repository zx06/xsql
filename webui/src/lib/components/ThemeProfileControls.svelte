<script>
  let {
    themeMode = 'auto',
    selectedProfile = '',
    profiles = [],
    authRequired = false,
    authToken = '',
    disabled = false,
    onThemeChange,
    onProfileChange,
    onTokenChange
  } = $props();
</script>

<div class="grid gap-3">
  <label class="grid gap-1.5 text-xs text-[var(--muted)]">
    <span>Theme</span>
    <select class="xsql-input" value={themeMode} onchange={(event) => onThemeChange?.(event.currentTarget.value)}>
      <option value="auto">Auto</option>
      <option value="white">White</option>
      <option value="black">Black</option>
    </select>
  </label>

  <label class="grid gap-1.5 text-xs text-[var(--muted)]">
    <span>Profile</span>
    <select
      class="xsql-input"
      value={selectedProfile}
      disabled={disabled || profiles.length === 0}
      onchange={(event) => onProfileChange?.(event.currentTarget.value)}
    >
      <option value="" disabled>Select a profile</option>
      {#each profiles as profile (profile.name)}
        <option value={profile.name}>{profile.name} · {profile.db}</option>
      {/each}
    </select>
  </label>

  {#if authRequired}
    <label class="grid gap-1.5 text-xs text-[var(--muted)]">
      <span>Token</span>
      <input
        class="xsql-input"
        type="password"
        value={authToken}
        placeholder="Bearer token"
        onchange={(event) => onTokenChange?.(event.currentTarget.value)}
      />
    </label>
  {/if}
</div>
