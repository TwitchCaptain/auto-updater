<script lang="ts">
  import { onMount } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import { Service } from '../bindings/github.com/TwitchCaptain/auto-updater/internal/service';

  type Slot = { days: number[]; time: string; action: string };
  type AppRow = {
    id: string;
    name: string;
    enabled: boolean;
    source: string;
    ownerRepo: string;
    exePath: string;
    shortcutPath?: string;
    extraFiles?: string[];
    versionHttp?: string;
    versionJson?: string;
    versionHttpHeader?: string;
    versionHttpSecret?: string;
    lastVersion?: string;
    schedules: Slot[];
  };
  type Settings = { githubToken?: string; startWithWindows: boolean; apps: AppRow[] };
  type Preview = {
    tag_name: string;
    html_url: string;
    asset: { name: string; browser_download_url: string; size: number };
    newer?: boolean;
  };
  type Hist = {
    time: string;
    appId: string;
    appName: string;
    action: string;
    from?: string;
    to?: string;
    asset?: string;
    result?: string;
    error?: string;
  };
  type Preset = {
    id: string;
    name: string;
    ownerRepo: string;
    primaryExe: string;
    extraFiles?: string[];
    notes: string;
  };
  type Shortcut = { target: string; arguments: string; workingDir: string; icon: string; description: string };
  type Upcoming = { time: string; appId: string; appName: string; action: string };
  type DataLoc = { dir: string; config: string; history: string };
  type Page = 'apps' | 'activity' | 'settings';

  const days = [
    { n: 0, l: 'Sun' },
    { n: 1, l: 'Mon' },
    { n: 2, l: 'Tue' },
    { n: 3, l: 'Wed' },
    { n: 4, l: 'Thu' },
    { n: 5, l: 'Fri' },
    { n: 6, l: 'Sat' },
  ];

  let page = $state<Page>('apps');
  let locked = $state(false);
  let encrypted = $state(false);
  let password = $state('');
  let settings = $state<Settings>({ startWithWindows: false, apps: [] });
  let selected = $state<AppRow | null>(null);
  let preview = $state<Preview | null>(null);
  let localVer = $state('');
  let shortcut = $state<Shortcut | null>(null);
  let past = $state<Hist[]>([]);
  let upcoming = $state<Upcoming[]>([]);
  let presets = $state<Preset[]>([]);
  let dataPaths = $state<DataLoc | null>(null);
  let err = $state('');
  let ok = $state('');
  let busy = $state('');
  let extraText = $state('');
  let wouldWrite = $state<string[]>([]);
  let highlightUpgrade = $state(false);
  let okTimer: ReturnType<typeof setTimeout> | undefined;

  const extraList = $derived(
    extraText
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean),
  );
  const pastNewest = $derived((past ?? []).toReversed());

  function go(next: Page) {
    page = next;
    if (next === 'activity' || next === 'settings') void refresh();
  }

  function formatWhen(iso: string) {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso || '';
    return d.toLocaleString();
  }

  function relative(iso: string) {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '';
    const sec = (Date.now() - d.getTime()) / 1000;
    if (sec < 45) return 'just now';
    if (sec < 3600) return `${Math.max(1, Math.floor(sec / 60))}m ago`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
    if (sec < 86400 * 7) return `${Math.floor(sec / 86400)}d ago`;
    return d.toLocaleDateString();
  }

  function actionLabel(a: string) {
    if (a === 'upgrade') return 'Upgrade';
    if (a === 'notify') return 'Notify';
    if (a === 'check') return 'Check';
    if (a === 'error') return 'Error';
    return a;
  }

  function resultLabel(h: Hist) {
    if (h.error) return h.error;
    if (h.result === 'current') return 'already up to date';
    if (h.result === 'ok') return 'ok';
    if (h.result === 'newer=true') return 'update available';
    if (h.result === 'newer=false') return 'up to date';
    return h.result || h.asset || '';
  }

  function lastHist(id: string) {
    return pastNewest.find((h) => h.appId === id);
  }

  function nextSlot(id: string) {
    return upcoming.find((u) => u.appId === id);
  }

  function histLine(h: Hist) {
    const when = relative(h.time);
    if (h.action === 'error') return `Last run failed ${when}`;
    if (h.action === 'upgrade' && h.result === 'ok') return `Upgraded to ${h.to || ''} ${when}`.trim();
    if (h.action === 'upgrade' && h.result === 'current') return `Up to date ${when}`;
    if (h.action === 'check' && h.result === 'newer=true') return `Update available (${h.to || ''}) ${when}`.trim();
    if (h.action === 'check') return `Checked ${when}`;
    if (h.action === 'notify' && h.result === 'current') return `Notify: already current ${when}`;
    if (h.action === 'notify') return `Notified ${when}`;
    return `${actionLabel(h.action)} ${when}`.trim();
  }

  function appStatusLine(a: AppRow) {
    const bits: string[] = [];
    if (a.lastVersion) bits.push(a.lastVersion);
    const h = lastHist(a.id);
    if (h) bits.push(histLine(h));
    else if (!a.lastVersion) bits.push('Not checked yet');
    const n = nextSlot(a.id);
    if (!a.enabled) bits.push('Disabled');
    else if (n) bits.push(`Next ${actionLabel(n.action)} ${formatWhen(n.time)}`);
    return bits.join(' · ');
  }

  function openApp(id: string) {
    const a = settings.apps.find((x) => x.id === id);
    page = 'apps';
    if (a) pick(a);
  }

  const emptyApp = (): AppRow => ({
    id: crypto.randomUUID(),
    name: '',
    enabled: true,
    source: 'github',
    ownerRepo: '',
    exePath: '',
    extraFiles: [],
    schedules: [],
  });

  function fail(e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    console.error(e);
    ok = '';
    err = msg;
  }

  function normalizeRepoLocal(raw: string) {
    let s = raw.trim().replace(/^["']+|["']+$/g, '').trim();
    if (!s) return '';
    if (s.startsWith('git@')) {
      const i = s.indexOf(':');
      if (i >= 0) s = s.slice(i + 1);
    }
    const lower = s.toLowerCase();
    const prefixes = [
      'git+https://github.com/',
      'https://github.com/',
      'http://github.com/',
      'https://www.github.com/',
      'http://www.github.com/',
      'github.com/',
      'www.github.com/',
    ];
    for (const p of prefixes) {
      if (lower.startsWith(p)) {
        s = s.slice(p.length);
        break;
      }
    }
    const cut = s.search(/[?#]/);
    if (cut >= 0) s = s.slice(0, cut);
    s = s.replace(/^\/+|\/+$/g, '').replace(/\.git$/i, '').replace(/\/+$/g, '');
    const parts = s.split('/');
    if (parts.length >= 2 && parts[0] && parts[1]) {
      return `${parts[0]}/${parts[1].replace(/\.git$/i, '')}`;
    }
    return s;
  }

  function noteOk(msg: string) {
    err = '';
    ok = msg;
    if (okTimer) clearTimeout(okTimer);
    okTimer = setTimeout(() => {
      if (ok === msg) ok = '';
    }, 12000);
  }

  function cloneApp(a: AppRow): AppRow {
    return JSON.parse(JSON.stringify(a)) as AppRow;
  }

  async function refreshWouldWrite() {
    if (!selected) {
      wouldWrite = [];
      return;
    }
    try {
      wouldWrite = (await Service.WouldWrite(selected.exePath, extraList)) ?? [];
    } catch {
      wouldWrite = [];
    }
  }

  async function refresh() {
    try {
      locked = await Service.NeedsUnlock();
      encrypted = await Service.Encrypted();
      try {
        dataPaths = await Service.DataPaths();
      } catch {
        dataPaths = null;
      }
      if (locked) return;
      const cfg = await Service.GetConfig();
      settings = { githubToken: cfg.githubToken, startWithWindows: !!cfg.startWithWindows, apps: (cfg.apps ?? []) as AppRow[] };
      past = (await Service.History(200)) ?? [];
      upcoming = (await Service.Upcoming()) ?? [];
      presets = ((await Service.Presets()) ?? []) as Preset[];
    } catch (e) {
      fail(e);
    }
  }

  async function unlock() {
    err = '';
    try {
      await Service.Unlock(password);
      password = '';
      await refresh();
    } catch (e) {
      fail(e);
    }
  }

  async function persist(): Promise<AppRow | null> {
    if (!selected || busy) return null;
    err = '';
    await normalizeRepoField();
    selected.extraFiles = extraList;
    busy = 'Saving…';
    try {
      const saved = await Service.SaveApp(selected);
      selected.id = saved.id;
      await refresh();
      const found = settings.apps.find((a) => a.id === saved.id);
      selected = found ? cloneApp(found) : selected;
      extraText = (selected.extraFiles || []).join('\n');
      return saved as AppRow;
    } catch (e) {
      fail(e);
      return null;
    } finally {
      busy = '';
    }
  }

  async function saveAll() {
    err = '';
    try {
      await Service.SaveConfig(settings);
      await refresh();
      noteOk('Settings saved.');
    } catch (e) {
      fail(e);
    }
  }

  async function saveApp() {
    const saved = await persist();
    if (saved) noteOk(`Saved ${saved.name || 'app'}.`);
  }

  function applyCheck(r: {
    local: string;
    remote: string;
    htmlUrl: string;
    assetName: string;
    assetURL: string;
    assetSize: number;
    wouldWrite?: string[] | null;
    newer?: boolean;
  }) {
    localVer = r.local;
    preview = {
      tag_name: r.remote,
      html_url: r.htmlUrl,
      asset: { name: r.assetName, browser_download_url: r.assetURL, size: r.assetSize },
      newer: r.newer,
    };
    wouldWrite = r.wouldWrite || wouldWrite;
  }

  async function checkNow() {
    if (!selected) return;
    const saved = await persist();
    if (!saved?.id) return;
    busy = 'Checking GitHub…';
    try {
      const r = await Service.CheckNow(saved.id);
      if (!r) {
        fail('Check returned nothing.');
        return;
      }
      applyCheck(r);
      await refresh();
      if (r.newer) {
        noteOk(`Update available for ${saved.name}: ${r.local || 'unknown'} → ${r.remote}.`);
      } else {
        noteOk(`${saved.name} is up to date (${r.remote || r.local}).`);
      }
    } catch (e) {
      fail(e);
    } finally {
      busy = '';
    }
  }

  async function upgradeNow() {
    if (!selected) return;
    const saved = await persist();
    if (!saved?.id) return;
    busy = 'Upgrading…';
    err = '';
    try {
      const r = await Service.UpgradeNow(saved.id);
      if (r?.restart) {
        busy = 'Restarting Captain Updater to finish the update…';
        noteOk('Restarting to finish the self-update.');
        return;
      }
      await refresh();
      if (r && !r.newer) {
        applyCheck(r);
        noteOk(`${saved.name} is already up to date (${r.local || r.remote}).`);
      } else if (r) {
        applyCheck({ ...r, local: r.remote, newer: false });
        noteOk(`Upgraded ${saved.name} ${r.local || 'unknown'} → ${r.remote}.`);
      } else {
        noteOk(`Upgrade finished for ${saved.name}.`);
      }
    } catch (e) {
      fail(e);
    } finally {
      busy = '';
    }
  }

  async function removeApp() {
    if (!selected) return;
    const id = selected.id;
    if (!id || !settings.apps.some((a) => a.id === id)) {
      selected = null;
      preview = null;
      localVer = '';
      extraText = '';
      wouldWrite = [];
      noteOk('Discarded unsaved app.');
      return;
    }
    await removeAppById(id);
  }

  async function removeAppById(id: string) {
    err = '';
    if (!id) {
      fail('This app has no id. Discard it with Delete, or save it first.');
      return;
    }
    try {
      const name = settings.apps.find((a) => a.id === id)?.name || 'app';
      await Service.DeleteApp(id);
      if (selected?.id === id) {
        selected = null;
        preview = null;
        localVer = '';
        extraText = '';
        wouldWrite = [];
      }
      await refresh();
      noteOk(`Deleted ${name}.`);
    } catch (e) {
      fail(e);
    }
  }

  async function normalizeRepoField() {
    if (!selected?.ownerRepo) return;
    const local = normalizeRepoLocal(selected.ownerRepo);
    if (local && local !== selected.ownerRepo) selected.ownerRepo = local;
    try {
      const remote = await Service.NormalizeRepo(selected.ownerRepo);
      if (remote && remote !== selected.ownerRepo) selected.ownerRepo = remote;
    } catch {
      /* local fallback already applied */
    }
  }

  async function doPreview() {
    if (!selected) return;
    await normalizeRepoField();
    if (!selected.ownerRepo) {
      fail('Enter a GitHub repo (owner/name) or paste a GitHub URL.');
      return;
    }
    err = '';
    busy = 'Checking GitHub…';
    try {
      preview = await Service.PreviewGitHub(selected.ownerRepo);
      try {
        localVer = await Service.LocalVersion(selected);
      } catch {
        localVer = selected.lastVersion || '';
      }
      await refreshWouldWrite();
      const latest = preview?.tag_name || '';
      if (localVer && latest) {
        noteOk(`GitHub latest is ${latest}. Installed: ${localVer}.`);
      } else if (latest) {
        noteOk(`GitHub latest is ${latest}.`);
      }
    } catch (e) {
      fail(e);
      preview = null;
    } finally {
      busy = '';
    }
  }

  async function loadLocal() {
    if (!selected) return;
    try {
      localVer = await Service.LocalVersion(selected);
    } catch {
      localVer = selected.lastVersion || '';
    }
    await refreshWouldWrite();
  }

  async function browseExe() {
    const p = await Service.BrowseExe();
    if (!p || !selected) return;
    if (p.toLowerCase().endsWith('.lnk')) {
      selected.shortcutPath = p;
      await loadLnk();
      return;
    }
    selected.exePath = p;
    await refreshWouldWrite();
    await loadLocal();
  }

  async function browseLnk() {
    const p = await Service.BrowseShortcut();
    if (p && selected) {
      selected.shortcutPath = p;
      await loadLnk();
    }
  }

  async function loadLnk() {
    if (!selected?.shortcutPath) {
      shortcut = null;
      return;
    }
    try {
      shortcut = await Service.ParseShortcut(selected.shortcutPath);
      if (
        shortcut?.target &&
        (!selected.exePath || selected.exePath.toLowerCase().endsWith('.lnk'))
      ) {
        selected.exePath = shortcut.target;
        await refreshWouldWrite();
        await loadLocal();
      }
    } catch {
      shortcut = null;
    }
  }

  async function applyPreset(id: string) {
    try {
      const a = await Service.ApplyPreset(id);
      selected = {
        id: crypto.randomUUID(),
        name: a.name,
        enabled: true,
        source: 'github',
        ownerRepo: a.ownerRepo,
        exePath: a.exePath || '',
        extraFiles: a.extraFiles || [],
        versionHttp: a.versionHttp,
        versionJson: a.versionJson,
        schedules: [],
      };
      extraText = (selected.extraFiles || []).join('\n');
      preview = null;
      localVer = '';
      page = 'apps';
      await refreshWouldWrite();
      noteOk(`Filled ${a.name} from template. Set the exe path, then Save.`);
    } catch (e) {
      fail(e);
    }
  }

  function addSchedule() {
    if (!selected) return;
    if (selected.schedules.length >= 5) return;
    selected.schedules = [...selected.schedules, { days: [1, 2, 3, 4, 5], time: '04:00', action: 'upgrade' }];
  }

  function toggleDay(slot: Slot, n: number) {
    if (slot.days.includes(n)) slot.days = slot.days.filter((d) => d !== n);
    else slot.days = [...slot.days, n];
  }

  function pick(a: AppRow, focusUpgrade = false) {
    try {
      selected = cloneApp(a);
      extraText = (a.extraFiles || []).join('\n');
      preview = null;
      localVer = a.lastVersion || '';
      highlightUpgrade = focusUpgrade;
      err = '';
      ok = '';
      void loadLnk();
      void refreshWouldWrite();
      if (focusUpgrade) {
        queueMicrotask(() => document.getElementById('upgrade-btn')?.focus());
      }
    } catch (e) {
      fail(e);
    }
  }

  function startAdd() {
    selected = emptyApp();
    extraText = '';
    preview = null;
    localVer = '';
    wouldWrite = [];
    shortcut = null;
    err = '';
    ok = '';
  }

  async function enableEnc() {
    err = '';
    try {
      await Service.EnableEncryption(password);
      password = '';
      await refresh();
      noteOk('Config encryption is on.');
    } catch (e) {
      fail(e);
    }
  }

  async function disableEnc() {
    err = '';
    try {
      await Service.DisableEncryption(password);
      password = '';
      await refresh();
      noteOk('Config encryption is off.');
    } catch (e) {
      fail(e);
    }
  }

  async function exportBackup() {
    err = '';
    try {
      const p = await Service.ExportBackup();
      if (p) noteOk(`Backup saved to ${p}`);
    } catch (e) {
      fail(e);
    }
  }

  async function openDataDir() {
    err = '';
    try {
      await Service.OpenDataDir();
      noteOk(`Opened ${dataPaths?.dir || 'data folder'} in Explorer.`);
    } catch (e) {
      fail(e);
    }
  }

  onMount(() => {
    const onErr = (e: ErrorEvent) => fail(e.error ?? e.message);
    const onRej = (e: PromiseRejectionEvent) => fail(e.reason);
    window.addEventListener('error', onErr);
    window.addEventListener('unhandledrejection', onRej);
    try {
      Events.On('config-locked', () => {
        locked = true;
        selected = null;
        settings = { startWithWindows: false, apps: [] };
        password = '';
        preview = null;
      });
      Events.On('open-app', (v: { data?: string }) => {
        const id = v?.data;
        if (!id) return;
        page = 'apps';
        const a = settings.apps.find((x) => x.id === id);
        if (a) pick(a, true);
      });
      Events.On('history-updated', () => {
        void Service.History(200).then((h) => (past = h ?? []));
        void Service.Upcoming().then((u) => (upcoming = u ?? []));
      });
      Events.On('files-dropped', (v) => {
        const files = v?.data || [];
        const exe = files.find((f) => f.toLowerCase().endsWith('.exe'));
        const lnk = files.find((f) => f.toLowerCase().endsWith('.lnk'));
        if ((exe || lnk) && !selected) {
          selected = emptyApp();
          extraText = '';
          preview = null;
          wouldWrite = [];
        }
        if (exe && selected) selected.exePath = exe;
        if (lnk && selected) {
          selected.shortcutPath = lnk;
          void loadLnk();
        }
        void refreshWouldWrite();
      });
    } catch {
      /* running outside WebView2 */
    }
    void (async () => {
      await refresh();
      try {
        const id = await Service.ConsumePendingOpen();
        if (!id) return;
        page = 'apps';
        const a = settings.apps.find((x) => x.id === id);
        if (a) pick(a, true);
      } catch {
        /* bindings may lag a rebuild; open-app event still works */
      }
    })();
    return () => {
      window.removeEventListener('error', onErr);
      window.removeEventListener('unhandledrejection', onRej);
      if (okTimer) clearTimeout(okTimer);
    };
  });
</script>

{#snippet editorActions(upgradeId: string)}
  <button type="button" class="btn btn-primary" disabled={!!busy} onclick={() => saveApp()}>Save</button>
  <button type="button" class="btn btn-info" disabled={!selected?.name || !!busy} onclick={() => checkNow()}
    >Check now</button
  >
  <button
    id={upgradeId || undefined}
    type="button"
    class={['btn', 'btn-warning', { 'shadow-lg': highlightUpgrade }]}
    disabled={!selected?.name || !!busy}
    onclick={() => upgradeNow()}>Upgrade</button
  >
  <button type="button" class="btn btn-outline-danger ms-auto" onclick={() => removeApp()}>Delete</button>
{/snippet}

{#if locked}
  <div class="container py-5" style="max-width: 28rem">
    <h1 class="h3 mb-3">Captain Updater</h1>
    <p class="text-body-secondary">Config is password-protected.</p>
    {#if err}<div class="alert alert-danger">{err}</div>{/if}
    <form
      onsubmit={(e) => {
        e.preventDefault();
        void unlock();
      }}
    >
      <input class="form-control mb-3" type="password" placeholder="Password" bind:value={password} />
      <button class="btn btn-primary w-100" type="submit">Unlock</button>
    </form>
  </div>
{:else}
  <div class="cu-chrome">
    <nav class="navbar navbar-expand navbar-dark bg-primary">
      <div class="container-fluid">
        <span class="navbar-brand">Captain Updater</span>
        <div class="navbar-nav flex-row">
          <button
            type="button"
            class={['nav-link', 'btn', 'btn-link', { active: page === 'apps' }]}
            onclick={() => go('apps')}
          >
            Apps
          </button>
          <button
            type="button"
            class={['nav-link', 'btn', 'btn-link', { active: page === 'activity' }]}
            onclick={() => go('activity')}
          >
            Activity
          </button>
          <button
            type="button"
            class={['nav-link', 'btn', 'btn-link', { active: page === 'settings' }]}
            onclick={() => go('settings')}
          >
            Settings
          </button>
        </div>
      </div>
    </nav>
    <div class="cu-status" role="status" aria-live="polite">
      {#if busy}
        <div class="alert alert-info py-2">{busy}</div>
      {:else if err}
        <div class="alert alert-danger py-2 d-flex align-items-start gap-2">
          <div class="flex-grow-1">{err}</div>
          <button type="button" class="btn-close btn-close-white" aria-label="Dismiss" onclick={() => (err = '')}
          ></button>
        </div>
      {:else if ok}
        <div class="alert alert-success py-2 d-flex align-items-start gap-2">
          <div class="flex-grow-1">{ok}</div>
          <button type="button" class="btn-close" aria-label="Dismiss" onclick={() => (ok = '')}></button>
        </div>
      {:else}
        <div class="small text-body-secondary py-1">
          Save, Check, and Upgrade report here so you do not have to scroll.
        </div>
      {/if}
    </div>
  </div>

  <div class="container-fluid py-3 pb-4">
    {#if page === 'apps'}
      <div class="row g-3">
        <div class="col-lg-4">
          <div class="d-flex justify-content-between mb-2">
            <h2 class="h5">Apps</h2>
            <button class="btn btn-sm btn-success" onclick={startAdd}>Add</button>
          </div>
          <div class="list-group mb-3">
            {#each settings.apps as a, i (a.id || `row-${i}`)}
              <div
                class={[
                  'list-group-item',
                  'd-flex',
                  'align-items-start',
                  'gap-2',
                  { active: selected?.id === a.id },
                ]}
              >
                <button
                  type="button"
                  class="btn btn-link text-start flex-grow-1 text-decoration-none p-0 text-reset"
                  onclick={() => pick(a)}
                >
                  <div class="fw-semibold">{a.name}</div>
                  <small class="text-body-secondary">{a.ownerRepo}</small>
                  <small class="d-block text-body-secondary text-truncate">{appStatusLine(a)}</small>
                </button>
                <button
                  type="button"
                  class="btn btn-sm btn-outline-danger"
                  title="Delete"
                  onclick={(e) => {
                    e.stopPropagation();
                    void removeAppById(a.id);
                  }}>×</button
                >
              </div>
            {/each}
            {#if settings.apps.length === 0}
              <div class="list-group-item text-body-secondary">No apps yet. Add one or use a template.</div>
            {/if}
          </div>
          <h3 class="h6">Templates</h3>
          {#each presets as p (p.id)}
            <button
              class="btn btn-outline-secondary btn-sm me-1 mb-1"
              title={p.notes}
              onclick={() => applyPreset(p.id)}>{p.name}</button
            >
          {/each}
        </div>
        <div class="col-lg-8">
          {#if selected}
            <div class="card">
              <div class="card-body">
                <div class="d-flex gap-2 flex-wrap mb-3">
                  {@render editorActions('')}
                </div>
                <div class="alert alert-secondary py-2">
                  <div>
                    Installed <strong>{localVer || selected.lastVersion || 'unknown'}</strong>
                    {#if preview}
                      · Latest <strong>{preview.tag_name}</strong>
                      {#if preview.newer}· <span class="text-warning">update available</span>{/if}
                    {:else}
                      · Latest unknown until you Check
                    {/if}
                  </div>
                  {#if lastHist(selected.id)}
                    <div class="small mt-1">{histLine(lastHist(selected.id)!)}</div>
                  {/if}
                  {#if nextSlot(selected.id) && selected.enabled}
                    <div class="small">
                      Next {actionLabel(nextSlot(selected.id)!.action)}
                      {formatWhen(nextSlot(selected.id)!.time)}
                    </div>
                  {:else if !selected.enabled}
                    <div class="small">Schedules are off while this app is disabled.</div>
                  {:else if selected.schedules.length === 0}
                    <div class="small">No schedule. Add one below, or use Check / Upgrade now.</div>
                  {/if}
                  {#if preview?.asset?.name}
                    <div class="small text-break mt-1">{preview.asset.name} ({preview.asset.size} bytes)</div>
                  {/if}
                  {#if wouldWrite.length}
                    <div class="small mt-1">Would write: {wouldWrite.join(', ')}</div>
                  {/if}
                </div>
                <div class="row g-2">
                  <div class="col-12">
                    <label class="form-label" for="app-name">Name</label>
                    <div class="input-group">
                      <div class="input-group-text" title="Enabled">
                        <input
                          id="en"
                          class="form-check-input mt-0 me-2"
                          type="checkbox"
                          bind:checked={selected.enabled}
                          aria-label="Enabled"
                        />
                        <label class="mb-0" for="en">Enabled</label>
                      </div>
                      <input id="app-name" class="form-control" bind:value={selected.name} />
                    </div>
                  </div>
                  <div class="col-12">
                    <label class="form-label" for="owner-repo">GitHub repo</label>
                    <div class="input-group">
                      <input
                        id="owner-repo"
                        class="form-control"
                        bind:value={selected.ownerRepo}
                        placeholder="owner/name or paste a GitHub URL"
                        onblur={(e) => {
                          const el = e.currentTarget;
                          if (selected && el instanceof HTMLInputElement) selected.ownerRepo = el.value;
                          void normalizeRepoField();
                        }}
                        onpaste={() => {
                          setTimeout(() => {
                            const el = document.getElementById('owner-repo');
                            if (selected && el instanceof HTMLInputElement) selected.ownerRepo = el.value;
                            void normalizeRepoField();
                          }, 0);
                        }}
                      />
                      <button type="button" class="btn btn-outline-info" onclick={() => doPreview()}
                        >Check repo</button
                      >
                    </div>
                    <div class="form-text">Paste a github.com URL; it is stored as owner/name.</div>
                  </div>
                  <div class="col-12">
                    <label class="form-label" for="exe-path">Primary exe (browse or drop)</label>
                    <div class="input-group" data-wails-dropzone>
                      <input
                        id="exe-path"
                        class="form-control"
                        bind:value={selected.exePath}
                        placeholder="C:\Program Files\app\app.exe"
                        onchange={() => refreshWouldWrite()}
                      />
                      <button class="btn btn-outline-secondary" onclick={() => browseExe()}>Browse</button>
                    </div>
                  </div>
                  <div class="col-12">
                    <label class="form-label" for="extra-files">Also copy from archive (one filename or glob per line)</label>
                    <textarea
                      id="extra-files"
                      class="form-control"
                      rows="2"
                      bind:value={extraText}
                      placeholder="sidecar.exe"
                      onchange={() => refreshWouldWrite()}
                    ></textarea>
                  </div>
                  <div class="col-12">
                    <label class="form-label" for="shortcut-path">Shortcut to start after upgrade (optional)</label>
                    <div class="input-group">
                      <input id="shortcut-path" class="form-control" bind:value={selected.shortcutPath} />
                      <button class="btn btn-outline-secondary" onclick={() => browseLnk()}>Browse</button>
                    </div>
                    {#if shortcut}
                      <div class="small mt-1 text-body-secondary">
                        Target: {shortcut.target}<br />
                        Args: {shortcut.arguments || '—'}<br />
                        Start in: {shortcut.workingDir || '—'}
                      </div>
                    {/if}
                  </div>
                  <div class="col-md-6">
                    <label class="form-label" for="ver-http">HTTP version URL (optional)</label>
                    <input
                      id="ver-http"
                      class="form-control"
                      bind:value={selected.versionHttp}
                      placeholder="http://127.0.0.1:7474/api/config"
                    />
                  </div>
                  <div class="col-md-3">
                    <label class="form-label" for="ver-json">JSON key</label>
                    <input id="ver-json" class="form-control" bind:value={selected.versionJson} placeholder="version" />
                  </div>
                  <div class="col-md-3">
                    <label class="form-label" for="ver-header">Header</label>
                    <input
                      id="ver-header"
                      class="form-control"
                      bind:value={selected.versionHttpHeader}
                      placeholder="X-API-Token"
                    />
                  </div>
                  <div class="col-12">
                    <label class="form-label" for="ver-secret">HTTP version secret (optional)</label>
                    <input id="ver-secret" class="form-control" type="password" bind:value={selected.versionHttpSecret} />
                  </div>
                </div>

                <h3 class="h6 mt-4">Schedules (up to 5)</h3>
                {#each selected.schedules as slot, i (`${i}-${slot.time}-${slot.action}`)}
                  <div class="border rounded p-2 mb-2">
                    <div class="d-flex flex-wrap gap-2 mb-2">
                      {#each days as d (d.n)}
                        <label
                          class={[
                            'btn',
                            'btn-sm',
                            slot.days.includes(d.n) ? 'btn-primary' : 'btn-outline-secondary',
                          ]}
                        >
                          <input
                            class="d-none"
                            type="checkbox"
                            checked={slot.days.includes(d.n)}
                            onchange={() => toggleDay(slot, d.n)}
                          />
                          {d.l}
                        </label>
                      {/each}
                    </div>
                    <div class="row g-2">
                      <div class="col-sm-4">
                        <input class="form-control" type="time" bind:value={slot.time} />
                      </div>
                      <div class="col-sm-4">
                        <select class="form-select" bind:value={slot.action}>
                          <option value="upgrade">Upgrade</option>
                          <option value="notify">Notify</option>
                        </select>
                      </div>
                      <div class="col-sm-4">
                        <button
                          class="btn btn-outline-danger"
                          onclick={() => (selected!.schedules = selected!.schedules.filter((_, j) => j !== i))}
                          >Remove</button
                        >
                      </div>
                    </div>
                  </div>
                {/each}
                <button class="btn btn-sm btn-outline-primary" disabled={selected.schedules.length >= 5} onclick={addSchedule}
                  >Add schedule</button
                >
              </div>
              <div class="cu-actions d-flex gap-2 flex-wrap">
                {@render editorActions('upgrade-btn')}
              </div>
            </div>
          {:else}
            <p class="text-body-secondary">Select an app or add one. Lives in the system tray until you open it.</p>
          {/if}
        </div>
      </div>
    {/if}

    {#if page === 'activity'}
      <h2 class="h5">Upcoming</h2>
      <p class="small text-body-secondary">Scheduled Upgrade and Notify times for enabled apps.</p>
      <div class="table-responsive mb-4">
        <table class="table table-sm table-striped">
          <thead>
            <tr>
              <th>When</th>
              <th>App</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {#each upcoming as u (`${u.time}-${u.appId}-${u.action}`)}
              <tr>
                <td class="text-nowrap">{formatWhen(u.time)}</td>
                <td>
                  <button type="button" class="btn btn-link btn-sm p-0 align-baseline" onclick={() => openApp(u.appId)}>
                    {u.appName}
                  </button>
                </td>
                <td>{actionLabel(u.action)}</td>
              </tr>
            {:else}
              <tr>
                <td colspan="3" class="text-body-secondary text-center py-4">- No upcoming actions -</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <h2 class="h5">History</h2>
      <p class="small text-body-secondary">Checks, upgrades, notifies, and errors — including “already up to date”.</p>
      <div class="table-responsive">
        <table class="table table-sm table-striped">
          <thead>
            <tr>
              <th>When</th>
              <th>App</th>
              <th>Action</th>
              <th>From</th>
              <th>To</th>
              <th>Detail</th>
            </tr>
          </thead>
          <tbody>
            {#each pastNewest as h (`${h.time}-${h.appId}-${h.action}-${h.from}-${h.to}-${h.error}`)}
              <tr>
                <td class="text-nowrap">{formatWhen(h.time)}</td>
                <td>{h.appName}</td>
                <td>{actionLabel(h.action)}</td>
                <td>{h.from || ''}</td>
                <td>{h.to || ''}</td>
                <td class="small">{resultLabel(h)}</td>
              </tr>
            {:else}
              <tr>
                <td colspan="6" class="text-body-secondary text-center py-4">- No history yet -</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

    {#if page === 'settings'}
      <div style="max-width: 40rem">
        <h2 class="h5">Settings</h2>
        <div class="mb-3">
          <label class="form-label" for="gh-token">GitHub token (optional)</label>
          <input id="gh-token" class="form-control" type="password" bind:value={settings.githubToken} />
          <div class="form-text">Raises API limits and allows private repos.</div>
        </div>
        <div class="form-check mb-3">
          <input class="form-check-input" type="checkbox" id="sww" bind:checked={settings.startWithWindows} />
          <label class="form-check-label" for="sww">Start with Windows</label>
        </div>
        <button class="btn btn-primary mb-4" onclick={() => saveAll()}>Save settings</button>

        <h3 class="h6">Data on disk</h3>
        <p class="small text-body-secondary">
          Apps, token, and schedules are in config.json. Activity is history.jsonl. Copy the folder or export a zip to
          back up.
        </p>
        <dl class="small">
          <dt>Folder</dt>
          <dd class="cu-path">{dataPaths?.dir || '%APPDATA%\\CaptainUpdater'}</dd>
          <dt>Config</dt>
          <dd class="cu-path">{dataPaths?.config || '%APPDATA%\\CaptainUpdater\\config.json'}</dd>
          <dt>History</dt>
          <dd class="cu-path">{dataPaths?.history || '%APPDATA%\\CaptainUpdater\\history.jsonl'}</dd>
        </dl>
        <div class="d-flex gap-2 flex-wrap mb-4">
          <button type="button" class="btn btn-secondary" onclick={() => openDataDir()}>Open folder</button>
          <button type="button" class="btn btn-primary" onclick={() => exportBackup()}>Export backup</button>
        </div>

        <h3 class="h6">Password protection</h3>
        <p class="small text-body-secondary">
          Encrypts config.json (tokens and paths). The activity log stays plaintext. A forgotten password means resetting
          config, not the log.
        </p>
        {#if encrypted}
          <p class="text-success">Encryption is on.</p>
          <input class="form-control mb-2" type="password" placeholder="Current password" bind:value={password} />
          <button class="btn btn-outline-warning" onclick={() => disableEnc()}>Turn off encryption</button>
        {:else}
          <input class="form-control mb-2" type="password" placeholder="New password" bind:value={password} />
          <button class="btn btn-outline-primary" onclick={() => enableEnc()}>Encrypt config</button>
        {/if}
      </div>
    {/if}
  </div>
{/if}
