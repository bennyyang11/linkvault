let bookmarks = [];
let tags = [];
let collections = [];
let licenseData = null;
let activeTag = '';
let activeCollection = null;
let searchTimeout = null;

document.addEventListener('DOMContentLoaded', init);

async function init() {
    await Promise.all([loadBookmarks(), loadTags(), loadCollections(), loadLicense(), checkUpdates()]);
    render();
    setInterval(checkUpdates, 5 * 60 * 1000);
    setInterval(loadLicense, 60 * 1000);
}

// --- API ---

async function loadBookmarks() {
    let url = '/api/bookmarks';
    const params = [];
    if (activeTag) params.push('tag=' + encodeURIComponent(activeTag));
    const q = document.getElementById('search-input')?.value.trim();
    if (q) params.push('q=' + encodeURIComponent(q));
    if (params.length) url += '?' + params.join('&');
    try {
        const resp = await fetch(url);
        if (resp.status === 403) {
            const data = await resp.json();
            showToast(data.error, 'error');
            return;
        }
        bookmarks = await resp.json();
    } catch (e) { console.error(e); }
}

async function loadTags() {
    try { tags = await (await fetch('/api/tags')).json(); } catch { tags = []; }
}

async function loadCollections() {
    try { collections = await (await fetch('/api/collections')).json(); } catch { collections = []; }
}

async function loadLicense() {
    try {
        licenseData = await (await fetch('/api/license')).json();
        renderLicense();
    } catch { licenseData = null; }
}

async function checkUpdates() {
    try {
        const data = await (await fetch('/api/updates')).json();
        const banner = document.getElementById('update-banner');
        if (data.available && data.update) {
            document.getElementById('update-text').textContent =
                '🚀 Update available: ' + data.update.versionLabel + ' — Click to dismiss';
            banner.classList.remove('hidden');
            banner.onclick = () => banner.classList.add('hidden');
        } else {
            banner.classList.add('hidden');
        }
    } catch {}
}

// --- Actions ---

async function handleAddBookmark(e) {
    e.preventDefault();
    const url = document.getElementById('add-url').value.trim();
    const tagsStr = document.getElementById('add-tags').value.trim();
    const colId = parseInt(document.getElementById('add-collection').value) || 0;
    const tagList = tagsStr ? tagsStr.split(',').map(t => t.trim()).filter(Boolean) : [];
    const btn = document.getElementById('add-submit');
    btn.disabled = true;
    btn.textContent = 'Saving…';
    try {
        const resp = await fetch('/api/bookmarks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url, tags: tagList, collection_id: colId })
        });
        const data = await resp.json();
        if (!resp.ok) { showToast(data.error || 'Failed to save', 'error'); return; }
        closeModal('add-modal');
        document.getElementById('add-url').value = '';
        document.getElementById('add-tags').value = '';
        document.getElementById('add-collection').value = '';
        showToast('Bookmark saved!');
        await Promise.all([loadBookmarks(), loadTags(), loadCollections()]);
        render();
    } catch { showToast('Error saving bookmark', 'error'); }
    finally { btn.disabled = false; btn.textContent = 'Save Bookmark'; }
}

async function deleteBookmark(id) {
    if (!confirm('Delete this bookmark?')) return;
    try {
        await fetch('/api/bookmarks/' + id, { method: 'DELETE' });
        await Promise.all([loadBookmarks(), loadTags()]);
        render();
        showToast('Bookmark deleted');
    } catch { showToast('Error deleting', 'error'); }
}

async function handleAddCollection(e) {
    e.preventDefault();
    const name = document.getElementById('col-name').value.trim();
    const desc = document.getElementById('col-desc').value.trim();
    try {
        const resp = await fetch('/api/collections', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, description: desc })
        });
        if (!resp.ok) { showToast('Failed to create', 'error'); return; }
        closeModal('collection-modal');
        document.getElementById('col-name').value = '';
        document.getElementById('col-desc').value = '';
        await loadCollections();
        render();
        showToast('Collection created!');
    } catch { showToast('Error creating collection', 'error'); }
}

async function toggleShare(id, event) {
    event.stopPropagation();
    try {
        const resp = await fetch('/api/collections/' + id + '/share', { method: 'PUT' });
        const data = await resp.json();
        if (!resp.ok) { showToast(data.error || 'Failed', 'error'); return; }
        if (data.is_public && data.share_code) {
            const link = window.location.origin + '/shared/' + data.share_code;
            navigator.clipboard.writeText(link).then(() => showToast('Share link copied to clipboard!'));
        } else {
            showToast('Collection is now private');
        }
        await loadCollections();
        render();
    } catch { showToast('Error', 'error'); }
}

function filterByTag(e, tag) {
    e.preventDefault();
    activeTag = tag;
    activeCollection = null;
    loadBookmarks().then(render);
}

async function filterByCollection(id) {
    activeCollection = id;
    activeTag = '';
    try {
        const col = await (await fetch('/api/collections/' + id)).json();
        bookmarks = col.bookmarks || [];
        render();
    } catch { showToast('Error loading collection', 'error'); }
}

function handleSearch(query) {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(async () => {
        if (query && licenseData && !licenseData.features?.search) {
            document.getElementById('search-disabled').classList.remove('hidden');
            return;
        }
        document.getElementById('search-disabled').classList.add('hidden');
        await loadBookmarks();
        render();
    }, 300);
}

async function generateSupportBundle() {
    const status = document.getElementById('bundle-status');
    status.textContent = 'Generating…';
    status.classList.remove('hidden');
    try {
        const resp = await fetch('/api/support-bundle', { method: 'POST' });
        const data = await resp.json();
        status.textContent = resp.ok ? '✓ ' + data.message : '✗ ' + (data.error || 'Failed');
    } catch { status.textContent = '✗ Error'; }
    setTimeout(() => status.classList.add('hidden'), 5000);
}

function showUpgradeToast(feature) {
    showToast('🔒 ' + feature.charAt(0).toUpperCase() + feature.slice(1) + ' requires a Pro plan', 'info');
}

// --- Render ---

function render() {
    renderBookmarks();
    renderTags();
    renderCollections();
    renderCollectionSelect();
}

function renderBookmarks() {
    const list = document.getElementById('bookmark-list');

    if (!bookmarks || !bookmarks.length) {
        const isFiltered = activeTag || (document.getElementById('search-input')?.value);
        list.innerHTML = `
            <div class="flex flex-col items-center justify-center py-20 text-center">
                <div class="w-14 h-14 bg-slate-100 rounded-2xl flex items-center justify-center mb-4">
                    <svg class="w-7 h-7 text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"/>
                    </svg>
                </div>
                <p class="text-sm font-medium text-slate-900 mb-1">${isFiltered ? 'No bookmarks found' : 'No bookmarks yet'}</p>
                <p class="text-sm text-slate-400">${isFiltered ? 'Try a different search or tag' : 'Click <strong>Add Bookmark</strong> to get started'}</p>
            </div>`;
        return;
    }

    list.innerHTML = bookmarks.map(b => {
        const domain = extractDomain(b.url);
        const date = timeAgo(b.created_at);
        const tagsHtml = (b.tags || []).map(t =>
            `<button onclick="filterByTag(event,'${esc(t)}')" class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-brand-50 text-brand-700 hover:bg-brand-100 transition-colors">#${esc(t)}</button>`
        ).join('');

        const faviconHtml = b.favicon_url
            ? `<img src="${esc(b.favicon_url)}" class="w-4 h-4 rounded flex-shrink-0" onerror="this.outerHTML='<div class=\\'w-4 h-4 rounded bg-slate-200 flex-shrink-0\\'></div>'">`
            : `<div class="w-4 h-4 rounded bg-slate-200 flex-shrink-0"></div>`;

        return `
        <div class="bookmark-card group bg-white border border-slate-200 hover:border-slate-300 rounded-xl p-4 transition-all hover:shadow-sm">
            <div class="flex items-start gap-3">
                <div class="mt-0.5">${faviconHtml}</div>
                <div class="flex-1 min-w-0">
                    <div class="flex items-start justify-between gap-2">
                        <a href="${esc(b.url)}" target="_blank" rel="noopener noreferrer"
                            class="text-sm font-semibold text-slate-900 hover:text-brand-600 transition-colors truncate block">
                            ${esc(b.title || b.url)}
                        </a>
                        <button onclick="deleteBookmark(${b.id})" class="delete-btn flex-shrink-0 p-1 text-slate-300 hover:text-red-500 hover:bg-red-50 rounded-lg transition-colors" title="Delete">
                            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
                        </button>
                    </div>
                    <p class="text-xs text-slate-400 truncate mt-0.5">${esc(domain)}</p>
                    ${b.description ? `<p class="text-xs text-slate-500 mt-1.5 line-clamp-2">${esc(b.description)}</p>` : ''}
                    <div class="flex items-center gap-1.5 mt-2 flex-wrap">
                        ${tagsHtml}
                        <span class="text-xs text-slate-300 ml-auto">${date}</span>
                    </div>
                </div>
            </div>
        </div>`;
    }).join('');
}

function renderTags() {
    const list = document.getElementById('tag-list');
    const total = bookmarks.length;

    const allActive = activeTag === '' && !activeCollection;
    let html = navItem(allActive, `filterByTag(event,'')`, `
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-4-7 4V5z"/></svg>
        All Bookmarks`, total);

    tags.forEach(t => {
        html += navItem(activeTag === t.name, `filterByTag(event,'${esc(t.name)}')`,
            `<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-5 5a2 2 0 01-2.828 0l-7-7A2 2 0 013 8V5a2 2 0 012-2z"/></svg>
            ${esc(t.name)}`, t.count);
    });
    list.innerHTML = html;
}

function navItem(active, onclick, content, count) {
    const base = 'flex items-center gap-2 px-2 py-1.5 rounded-lg text-sm cursor-pointer select-none transition-colors w-full text-left';
    const activeClass = active ? 'bg-brand-50 text-brand-700 font-medium' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900';
    return `<button onclick="${onclick}" class="${base} ${activeClass}">
        ${content}
        <span class="ml-auto text-xs ${active ? 'text-brand-500' : 'text-slate-300'}">${count}</span>
    </button>`;
}

function renderCollections() {
    const list = document.getElementById('collection-list');
    if (!collections.length) {
        list.innerHTML = '<p class="text-xs text-slate-400 px-2 py-1">No collections yet</p>';
        return;
    }
    list.innerHTML = collections.map(c => {
        const active = activeCollection === c.id;
        const base = 'group flex items-center gap-2 px-2 py-1.5 rounded-lg text-sm cursor-pointer select-none transition-colors w-full text-left';
        const cls = active ? 'bg-brand-50 text-brand-700 font-medium' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900';
        return `<button onclick="filterByCollection(${c.id})" class="${base} ${cls}">
            ${c.is_public
                ? `<svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064"/></svg>`
                : `<svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/></svg>`
            }
            <span class="truncate flex-1">${esc(c.name)}</span>
            <button onclick="toggleShare(${c.id}, event)" class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-slate-200 transition-all flex-shrink-0" title="${c.is_public ? 'Make private' : 'Get share link'}">
                ${c.is_public
                    ? `<svg class="w-3.5 h-3.5 text-brand-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"/></svg>`
                    : `<svg class="w-3.5 h-3.5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"/></svg>`
                }
            </button>
        </button>`;
    }).join('');
}

function renderCollectionSelect() {
    const sel = document.getElementById('add-collection');
    sel.innerHTML = '<option value="">None</option>' +
        collections.map(c => `<option value="${c.id}">${esc(c.name)}</option>`).join('');
}

function renderLicense() {
    const el = document.getElementById('license-info');
    const expiryBanner = document.getElementById('expiry-banner');
    const overlay = document.getElementById('expired-overlay');

    if (!licenseData || !licenseData.loaded) {
        el.innerHTML = `<div class="text-xs text-slate-400 bg-slate-50 rounded-lg p-2.5">Not connected to SDK</div>`;
        return;
    }

    licenseData.expired ? overlay.classList.remove('hidden') : overlay.classList.add('hidden');

    if (licenseData.days_until_expiry > 0 && licenseData.days_until_expiry < 30) {
        document.getElementById('expiry-text').textContent = `⚠️ License expires in ${licenseData.days_until_expiry} days`;
        expiryBanner.classList.remove('hidden');
    } else {
        expiryBanner.classList.add('hidden');
    }

    if (licenseData.features && !licenseData.features.search) {
        document.getElementById('search-disabled').classList.remove('hidden');
    }

    const f = licenseData.fields;
    const used = licenseData.enforcement?.bookmarks_used || 0;
    const limit = f.max_bookmarks || 0;
    const pct = limit > 0 ? Math.min(100, (used / limit) * 100) : 0;
    const barColor = pct > 80 ? 'bg-red-500' : 'bg-brand-500';

    const tier = f.feature_tier || 'unknown';
    const tierColor = tier === 'enterprise' ? 'bg-purple-100 text-purple-700'
        : tier === 'pro' ? 'bg-blue-100 text-blue-700'
        : 'bg-slate-100 text-slate-600';

    const features = [
        { name: 'Search', key: 'search' },
        { name: 'Sharing', key: 'public_collections' },
        { name: 'Export', key: 'import_export' },
    ];

    el.innerHTML = `
        <div class="bg-slate-50 rounded-xl p-3 space-y-3">
            <div class="flex items-center justify-between">
                <span class="text-xs text-slate-500">Plan</span>
                <span class="text-xs font-semibold px-2 py-0.5 rounded-full capitalize ${tierColor}">${esc(tier)}</span>
            </div>
            ${limit > 0 ? `
            <div>
                <div class="flex justify-between text-xs text-slate-500 mb-1">
                    <span>Bookmarks</span>
                    <span>${used} / ${limit}</span>
                </div>
                <div class="h-1.5 bg-slate-200 rounded-full overflow-hidden">
                    <div class="h-full ${barColor} rounded-full transition-all" style="width:${pct}%"></div>
                </div>
            </div>` : ''}
            <div class="grid grid-cols-3 gap-1">
                ${features.map(feat => {
                    const on = licenseData.features?.[feat.key];
                    return `<div class="flex flex-col items-center gap-0.5 p-1.5 rounded-lg ${on ? 'bg-green-50' : 'bg-slate-100'}">
                        <svg class="w-3 h-3 ${on ? 'text-green-500' : 'text-slate-300'}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            ${on ? '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/>'
                                 : '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M6 18L18 6M6 6l12 12"/>'}
                        </svg>
                        <span class="text-xs ${on ? 'text-green-600' : 'text-slate-400'}">${feat.name}</span>
                    </div>`;
                }).join('')}
            </div>
        </div>`;
}

// --- Modals ---

function showAddModal() {
    document.getElementById('add-modal').classList.remove('hidden');
    setTimeout(() => document.getElementById('add-url').focus(), 50);
}

function showCollectionModal() {
    document.getElementById('collection-modal').classList.remove('hidden');
    setTimeout(() => document.getElementById('col-name').focus(), 50);
}

function closeModal(id) {
    document.getElementById(id).classList.add('hidden');
}

function toggleAdmin() {
    const p = document.getElementById('admin-panel');
    p.classList.toggle('hidden');
    if (!p.classList.contains('hidden')) renderLicense();
}

// --- Toast ---

let toastTimer;
function showToast(msg, type = 'success') {
    const toast = document.getElementById('toast');
    const text = document.getElementById('toast-text');
    text.textContent = msg;
    toast.className = `fixed bottom-5 right-5 z-50 flex items-center gap-2 text-sm px-4 py-3 rounded-xl shadow-lg max-w-sm transition-all ${
        type === 'error' ? 'bg-red-600 text-white' :
        type === 'info'  ? 'bg-slate-700 text-white' :
                           'bg-slate-900 text-white'
    }`;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toast.classList.add('hidden'), 3500);
}

// --- Utils ---

function extractDomain(url) {
    try { return new URL(url).hostname; } catch { return url; }
}

function timeAgo(dateStr) {
    const diff = Date.now() - new Date(dateStr).getTime();
    const m = Math.floor(diff / 60000);
    const h = Math.floor(diff / 3600000);
    const d = Math.floor(diff / 86400000);
    if (m < 2) return 'just now';
    if (m < 60) return `${m}m ago`;
    if (h < 24) return `${h}h ago`;
    if (d < 30) return `${d}d ago`;
    return new Date(dateStr).toLocaleDateString();
}

function esc(str) {
    if (!str) return '';
    const d = document.createElement('div');
    d.textContent = str;
    return d.innerHTML;
}
