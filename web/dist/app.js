// State
let currentView = 'buckets';
let currentBucket = null;
let currentPrefix = '';
let currentUser = null;

// DOM Elements
const loginScreen = document.getElementById('login-screen');
const dashboardScreen = document.getElementById('dashboard-screen');
const loginForm = document.getElementById('login-form');
const loginError = document.getElementById('login-error');
const btnLogout = document.getElementById('btn-logout');
const navLinks = document.querySelectorAll('.nav-links li');
const views = document.querySelectorAll('.view');
const breadcrumb = document.getElementById('breadcrumb');
const topbarActions = document.getElementById('topbar-actions');

// API helper — uses cookies (JWT HttpOnly), no SigV4 needed
async function api(method, path, body = null) {
    const opts = { method, credentials: 'same-origin' };
    if (body) {
        opts.body = JSON.stringify(body);
        opts.headers = { 'Content-Type': 'application/json' };
    }
    const res = await fetch(path, opts);
    if (!res.ok) {
        const text = await res.text();
        let msg = text;
        try { msg = JSON.parse(text).error || text; } catch {}
        throw new Error(msg);
    }
    if (res.status === 204) return null;
    return res.json();
}

// S3 API helper — uses SigV4 via admin presign, or direct fetch with cookie proxy
async function s3Fetch(path) {
    const res = await fetch(path, { credentials: 'same-origin' });
    if (!res.ok) throw new Error(`S3 Error: ${res.status}`);
    return res;
}

// Initialization
document.addEventListener('DOMContentLoaded', async () => {
    try {
        currentUser = await api('GET', '/api/v1/me');
        showDashboard();
    } catch {
        showLogin();
    }
});

function showLogin() {
    loginScreen.classList.add('active');
    dashboardScreen.classList.remove('active');
}

function showDashboard() {
    loginScreen.classList.remove('active');
    dashboardScreen.classList.add('active');
    if (currentUser) {
        const el = document.getElementById('user-display');
        if (el) el.textContent = currentUser.username;
    }
    loadView('buckets');
}

// Login
loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    loginError.textContent = '';
    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;

    try {
        currentUser = await api('POST', '/api/v1/login', { username, password });
        showDashboard();
        showToast('Connected to GoS3', 'success');
    } catch (err) {
        loginError.textContent = err.message;
    }
});

// Logout
btnLogout.addEventListener('click', async () => {
    try { await api('POST', '/api/v1/logout'); } catch {}
    currentUser = null;
    showLogin();
});

// Navigation
navLinks.forEach(link => {
    link.addEventListener('click', () => {
        const view = link.dataset.view;
        navLinks.forEach(l => l.classList.remove('active'));
        link.classList.add('active');
        loadView(view);
    });
});

function loadView(viewName) {
    currentView = viewName;
    views.forEach(v => v.classList.remove('active'));
    const el = document.getElementById(`view-${viewName}`);
    if (el) el.classList.add('active');

    topbarActions.innerHTML = '';
    if (viewName === 'buckets') {
        currentBucket = null;
        currentPrefix = '';
        breadcrumb.innerHTML = `<span>Buckets</span>`;
        topbarActions.innerHTML = `<button class="btn btn-primary" onclick="openModal('create-bucket-modal')"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg> Create Bucket</button>`;
        fetchBuckets();
    } else if (viewName === 'users') {
        breadcrumb.innerHTML = `<span>IAM Users</span>`;
        fetchUsers();
    } else if (viewName === 'service-accounts') {
        breadcrumb.innerHTML = `<span>Service Accounts</span>`;
        topbarActions.innerHTML = `<button class="btn btn-primary" onclick="openModal('create-sa-modal')"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg> Create Access Key</button>`;
        fetchServiceAccounts();
    } else if (viewName === 'info') {
        breadcrumb.innerHTML = `<span>Server Info</span>`;
        fetchServerInfo();
    }
}

// ==================== Buckets ====================
async function fetchBuckets() {
    const grid = document.getElementById('buckets-grid');
    grid.innerHTML = '<div class="loading-spinner"></div>';
    try {
        const data = await api('GET', '/_admin/buckets');
        // Fallback: if admin endpoint not available, use info
        renderBucketCards(grid, data || []);
    } catch {
        // Fallback: try S3 XML list (requires SigV4, may not work from browser)
        grid.innerHTML = '<div style="grid-column:1/-1;text-align:center;color:var(--text-muted);padding:2rem;">Cannot list buckets. S3 API requires SigV4 which is not available in browser session mode.<br>Use Service Accounts with AWS CLI instead.</div>';
    }
}

function renderBucketCards(grid, buckets) {
    if (!buckets || buckets.length === 0) {
        grid.innerHTML = '<div style="grid-column:1/-1;text-align:center;color:var(--text-muted);padding:2rem;">No buckets found. Create one to get started!</div>';
        return;
    }
    grid.innerHTML = buckets.map(b => {
        const name = b.name || b.Name || b;
        const creationDate = b.creationDate ? new Date(b.creationDate).toLocaleString() : '';
        return `
            <div class="bucket-card" onclick="openBucket('${name}')">
                <div class="bucket-header">
                    <div class="bucket-icon">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path></svg>
                    </div>
                    <div class="bucket-info">
                        <h3>${name}</h3>
                        ${creationDate ? `<p>Created: ${creationDate}</p>` : ''}
                    </div>
                </div>
                <div class="bucket-actions">
                    <button class="btn btn-ghost" onclick="event.stopPropagation(); deleteBucket('${name}')">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                    </button>
                </div>
            </div>`;
    }).join('');
}

window.openBucket = (name) => {
    currentBucket = name;
    currentPrefix = '';
    updateBreadcrumb();
    topbarActions.innerHTML = `<button class="btn btn-primary" onclick="document.getElementById('file-input').click()"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="17 8 12 3 7 8"></polyline><line x1="12" y1="3" x2="12" y2="15"></line></svg> Upload</button>
    <button class="btn btn-ghost" onclick="createFolder()"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path><line x1="12" y1="11" x2="12" y2="17"></line><line x1="9" y1="14" x2="15" y2="14"></line></svg> New Folder</button>`;
    views.forEach(v => v.classList.remove('active'));
    document.getElementById('view-objects').classList.add('active');
    fetchObjects();
};

function updateBreadcrumb() {
    let html = `<span class="clickable" onclick="loadView('buckets')">Buckets</span>`;
    html += `<span class="separator">/</span>`;
    html += `<span class="clickable" onclick="navigatePrefix('')" style="color:var(--accent-primary)">${currentBucket}</span>`;
    if (currentPrefix) {
        const parts = currentPrefix.split('/').filter(Boolean);
        let accumulated = '';
        parts.forEach(p => {
            accumulated += p + '/';
            const prefixCopy = accumulated;
            html += `<span class="separator">/</span><span class="clickable" onclick="navigatePrefix('${prefixCopy}')">${p}</span>`;
        });
    }
    breadcrumb.innerHTML = html;
}

window.navigatePrefix = (prefix) => {
    currentPrefix = prefix;
    updateBreadcrumb();
    fetchObjects();
};

document.getElementById('create-bucket-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('new-bucket-name').value;
    try {
        await api('PUT', `/_admin/buckets/${name}`);
        showToast(`Bucket ${name} created`, 'success');
        closeModal('create-bucket-modal');
        document.getElementById('new-bucket-name').value = '';
        fetchBuckets();
    } catch (err) {
        showToast(err.message, 'error');
    }
});

window.deleteBucket = async (name) => {
    if (!confirm(`Delete bucket ${name}?`)) return;
    try {
        await api('DELETE', `/_admin/buckets/${name}`);
        showToast(`Bucket ${name} deleted`, 'success');
        fetchBuckets();
    } catch (err) {
        showToast(err.message, 'error');
    }
};

// ==================== Objects ====================
let objectsCache = [];
let sortField = 'key';
let sortAsc = true;

async function fetchObjects() {
    const tbody = document.getElementById('objects-table-body');
    tbody.innerHTML = '<tr><td colspan="4" style="text-align:center;"><div class="loading-spinner" style="margin:1rem auto;width:24px;height:24px;"></div></td></tr>';
    try {
        const data = await api('GET', `/_admin/buckets/${currentBucket}/objects?prefix=${encodeURIComponent(currentPrefix)}&delimiter=/`);
        objectsCache = [];
        // Add folders (common prefixes)
        if (data.commonPrefixes) {
            data.commonPrefixes.forEach(p => {
                objectsCache.push({ key: p, isFolder: true, size: 0, lastModified: '' });
            });
        }
        // Add files
        if (data.contents) {
            data.contents.forEach(obj => {
                if (!obj.key.endsWith('/')) {
                    objectsCache.push({ key: obj.key, isFolder: false, size: obj.size, lastModified: obj.lastModified });
                }
            });
        }
        renderObjects();
    } catch {
        tbody.innerHTML = '<tr><td colspan="4" style="text-align:center;color:var(--text-muted);">Could not load objects. Admin API may not support this yet.</td></tr>';
    }
}

function renderObjects() {
    const tbody = document.getElementById('objects-table-body');
    const searchVal = (document.getElementById('object-search')?.value || '').toLowerCase();
    let filtered = objectsCache.filter(o => {
        const displayName = o.isFolder ? o.key.replace(currentPrefix, '').replace(/\/$/, '') : o.key.replace(currentPrefix, '');
        return displayName.toLowerCase().includes(searchVal);
    });
    // Sort
    filtered.sort((a, b) => {
        if (a.isFolder !== b.isFolder) return a.isFolder ? -1 : 1;
        let va, vb;
        if (sortField === 'key') { va = a.key; vb = b.key; }
        else if (sortField === 'size') { va = a.size; vb = b.size; }
        else { va = a.lastModified; vb = b.lastModified; }
        if (va < vb) return sortAsc ? -1 : 1;
        if (va > vb) return sortAsc ? 1 : -1;
        return 0;
    });

    if (filtered.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="text-align:center;color:var(--text-muted);">Empty — upload files or create a folder.</td></tr>';
        return;
    }

    tbody.innerHTML = filtered.map(obj => {
        const displayName = obj.isFolder ? obj.key.replace(currentPrefix, '').replace(/\/$/, '') : obj.key.replace(currentPrefix, '');
        if (obj.isFolder) {
            return `<tr onclick="navigatePrefix('${obj.key}')" style="cursor:pointer;">
                <td><div class="file-name"><svg viewBox="0 0 24 24" fill="none" stroke="var(--accent-primary)" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path></svg> ${displayName}/</div></td>
                <td>—</td><td>—</td><td></td></tr>`;
        }
        const lastMod = obj.lastModified ? new Date(obj.lastModified).toLocaleString() : '';
        return `<tr>
            <td><div class="file-name"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path><polyline points="13 2 13 9 20 9"></polyline></svg> ${displayName}</div></td>
            <td>${formatBytes(obj.size)}</td>
            <td>${lastMod}</td>
            <td><div class="action-btns">
                <button class="btn btn-ghost" style="padding:0.4rem;" onclick="downloadObject('${obj.key}')" title="Download"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg></button>
                <button class="btn btn-ghost text-danger" style="padding:0.4rem;" onclick="deleteObject('${obj.key}')" title="Delete"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg></button>
            </div></td></tr>`;
    }).join('');
}

window.createFolder = async () => {
    const name = prompt('Folder name:');
    if (!name) return;
    try {
        await api('PUT', `/_admin/buckets/${currentBucket}/objects/${currentPrefix}${name}/`);
        showToast(`Folder ${name} created`, 'success');
        fetchObjects();
    } catch (err) { showToast(err.message, 'error'); }
};

window.downloadObject = async (key) => {
    try {
        const data = await api('POST', '/_admin/presign', { method: 'GET', bucket: currentBucket, key, expires: 3600 });
        window.open(data.url, '_blank');
    } catch (err) { showToast(err.message, 'error'); }
};

window.deleteObject = async (key) => {
    if (!confirm(`Delete ${key}?`)) return;
    try {
        await api('DELETE', `/_admin/buckets/${currentBucket}/objects/${key}`);
        showToast('Deleted', 'success');
        fetchObjects();
    } catch (err) { showToast(err.message, 'error'); }
};

// Upload
const uploadZone = document.getElementById('upload-zone');
const fileInput = document.getElementById('file-input');
if (uploadZone) {
    uploadZone.addEventListener('click', () => fileInput.click());
    uploadZone.addEventListener('dragover', (e) => { e.preventDefault(); uploadZone.classList.add('dragover'); });
    uploadZone.addEventListener('dragleave', () => uploadZone.classList.remove('dragover'));
    uploadZone.addEventListener('drop', (e) => { e.preventDefault(); uploadZone.classList.remove('dragover'); if (e.dataTransfer.files.length) handleUploads(e.dataTransfer.files); });
}
if (fileInput) fileInput.addEventListener('change', () => { if (fileInput.files.length) handleUploads(fileInput.files); });

async function handleUploads(files) {
    if (!currentBucket) { showToast('Select a bucket first', 'error'); return; }
    for (const file of files) {
        try {
            showToast(`Uploading ${file.name}...`, 'info');
            // Get presigned PUT URL from admin API
            const data = await api('POST', '/_admin/presign', { method: 'PUT', bucket: currentBucket, key: `${currentPrefix}${file.name}`, expires: 3600 });
            const res = await fetch(data.url, { method: 'PUT', body: file });
            if (!res.ok) throw new Error(`Upload failed: ${res.status}`);
            showToast(`${file.name} uploaded`, 'success');
        } catch (err) { showToast(err.message, 'error'); }
    }
    fetchObjects();
}

// ==================== Service Accounts ====================
async function fetchServiceAccounts() {
    const tbody = document.getElementById('sa-table-body');
    if (!tbody) return;
    tbody.innerHTML = '<tr><td colspan="4"><div class="loading-spinner"></div></td></tr>';
    try {
        const accounts = await api('GET', '/api/v1/service-accounts');
        tbody.innerHTML = accounts.map(sa => `
            <tr>
                <td><code>${sa.accessKeyId}</code></td>
                <td>${sa.description || '—'}</td>
                <td>${new Date(sa.createdAt).toLocaleString()}</td>
                <td><button class="btn btn-ghost text-danger" onclick="deleteServiceAccount('${sa.id}')">Delete</button></td>
            </tr>`).join('');
    } catch (err) { showToast(err.message, 'error'); tbody.innerHTML = ''; }
}

document.getElementById('create-sa-form')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const desc = document.getElementById('sa-description').value;
    try {
        const result = await api('POST', '/api/v1/service-accounts', { description: desc });
        closeModal('create-sa-modal');
        document.getElementById('sa-description').value = '';
        // Show the secret key (one time only)
        const infoDiv = document.getElementById('sa-created-info');
        if (infoDiv) {
            document.getElementById('sa-created-access').textContent = result.accessKeyId;
            document.getElementById('sa-created-secret').textContent = result.secretKey;
            document.getElementById('sa-created-endpoint').textContent = window.location.origin.replace(/:\d+$/, ':9010');
            openModal('sa-created-modal');
        }
        fetchServiceAccounts();
    } catch (err) { showToast(err.message, 'error'); }
});

window.deleteServiceAccount = async (id) => {
    if (!confirm('Delete this access key?')) return;
    try {
        await api('DELETE', `/api/v1/service-accounts/${id}`);
        showToast('Access key deleted', 'success');
        fetchServiceAccounts();
    } catch (err) { showToast(err.message, 'error'); }
};

window.copyToClipboard = (elementId) => {
    const text = document.getElementById(elementId)?.textContent;
    if (text) { navigator.clipboard.writeText(text); showToast('Copied!', 'success'); }
};

// ==================== Users ====================
async function fetchUsers() {
    const tbody = document.getElementById('users-table-body');
    tbody.innerHTML = '<tr><td colspan="3"><div class="loading-spinner"></div></td></tr>';
    try {
        const users = await api('GET', '/_admin/users');
        tbody.innerHTML = users.map(u => `
            <tr>
                <td><strong>${u.username}</strong>${u.isRoot ? ' <span class="text-accent" style="font-size:0.8rem;background:rgba(0,240,255,0.1);padding:2px 6px;border-radius:4px;margin-left:8px;">ROOT</span>' : ''}</td>
                <td>${u.disabled ? '<span style="color:var(--danger)">Disabled</span>' : '<span style="color:var(--success)">Active</span>'}</td>
                <td>${!u.isRoot ? `<button class="btn btn-ghost text-danger" onclick="deleteUser('${u.username}')">Delete</button>` : ''}</td>
            </tr>`).join('');
    } catch (err) { showToast('Admin API Error: ' + err.message, 'error'); tbody.innerHTML = ''; }
}

document.getElementById('create-user-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = document.getElementById('new-username').value;
    const password = document.getElementById('new-password').value;
    try {
        await api('POST', '/_admin/users', { username, password });
        showToast(`User ${username} created`, 'success');
        document.getElementById('new-username').value = '';
        document.getElementById('new-password').value = '';
        fetchUsers();
    } catch (err) { showToast(err.message, 'error'); }
});

window.deleteUser = async (username) => {
    if (!confirm(`Delete user ${username}?`)) return;
    try {
        await api('DELETE', `/_admin/users/${username}`);
        showToast(`User ${username} deleted`, 'success');
        fetchUsers();
    } catch (err) { showToast(err.message, 'error'); }
};

// ==================== Server Info ====================
async function fetchServerInfo() {
    const grid = document.getElementById('info-grid');
    grid.innerHTML = '<div class="loading-spinner"></div>';
    try {
        const info = await api('GET', '/_admin/info');
        const endpoint = window.location.origin.replace(/:\d+$/, ':9010');
        grid.innerHTML = `
            <div class="glass-card info-card"><span class="info-label">Version</span><span class="info-value" style="color:var(--accent-primary)">${info.version}</span></div>
            <div class="glass-card info-card"><span class="info-label">Uptime</span><span class="info-value">${formatDuration(info.uptime_seconds)}</span></div>
            <div class="glass-card info-card"><span class="info-label">Total Objects</span><span class="info-value">${info.storage?.total_objects || 0}</span></div>
            <div class="glass-card info-card"><span class="info-label">Total Storage</span><span class="info-value">${formatBytes(info.storage?.total_bytes || 0)}</span></div>
            <div class="glass-card info-card" style="grid-column:1/-1"><span class="info-label">API Endpoint</span><span class="info-value" style="font-family:monospace;font-size:1rem;user-select:all">${endpoint}</span></div>`;
    } catch (err) { showToast(err.message, 'error'); grid.innerHTML = ''; }
}

// ==================== Utils ====================
window.openModal = (id) => document.getElementById(id)?.classList.add('active');
window.closeModal = (id) => document.getElementById(id)?.classList.remove('active');

function showToast(msg, type = 'info') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    let icon = '';
    if (type === 'success') icon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:18px;height:18px"><polyline points="20 6 9 17 4 12"></polyline></svg>';
    else if (type === 'error') icon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:18px;height:18px"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>';
    else icon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:18px;height:18px"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>';
    toast.innerHTML = `${icon} <span>${msg}</span>`;
    container.appendChild(toast);
    setTimeout(() => { toast.style.animation = 'fadeOut 0.3s ease-out forwards'; setTimeout(() => toast.remove(), 300); }, 3000);
}

function formatBytes(bytes, decimals = 2) {
    if (!+bytes) return '0 Bytes';
    const k = 1024, dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

function formatDuration(seconds) {
    const d = Math.floor(seconds / (3600*24)), h = Math.floor(seconds % (3600*24) / 3600);
    const m = Math.floor(seconds % 3600 / 60), s = Math.floor(seconds % 60);
    if (d > 0) return `${d}d ${h}h`;
    if (h > 0) return `${h}h ${m}m`;
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
}

// Sort handler
window.sortObjects = (field) => {
    if (sortField === field) sortAsc = !sortAsc;
    else { sortField = field; sortAsc = true; }
    renderObjects();
};
