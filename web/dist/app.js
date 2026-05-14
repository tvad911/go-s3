// State
let currentView = 'buckets';
let currentBucket = null;
let currentPrefix = '';
let currentUser = null;
let selectedObjects = new Set();

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
    if (res.status === 204 || res.status === 201) {
        const text = await res.text();
        if (!text) return null;
        try { return JSON.parse(text); } catch { return text; }
    }
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
    } else if (viewName === 'connection') {
        breadcrumb.innerHTML = `<span>S3 Connection</span>`;
        loadConnectionInfo();
    } else if (viewName === 'info') {
        breadcrumb.innerHTML = `<span>Server Info</span>`;
        fetchServerInfo();
    } else if (viewName === 'audit-logs') {
        breadcrumb.innerHTML = `<span>Audit Logs</span>`;
        loadAuditLogs();
    } else if (viewName === 'policies') {
        breadcrumb.innerHTML = `<span>IAM Policies</span>`;
        fetchPolicies();
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
                    <button class="btn btn-primary btn-sm" onclick="event.stopPropagation(); openBucket('${name}')" title="Manage Files">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:16px;height:16px;margin-right:4px;display:inline-block;vertical-align:middle;"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path></svg>
                        Manage Files
                    </button>
                    <button class="btn btn-ghost" onclick="event.stopPropagation(); openBucketSettings('${name}')" title="Settings">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>
                    </button>
                    <button class="btn btn-ghost text-danger" onclick="event.stopPropagation(); deleteBucket('${name}')" title="Delete">
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

let currentPage = 1;
let objectsCache = [];

window.navigatePrefix = (prefix) => {
    currentPrefix = prefix;
    currentPage = 1;
    updateBreadcrumb();
    fetchObjects();
};

window.goToPage = (page) => {
    currentPage = page;
    renderObjects();
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

let sortField = 'key';
let sortAsc = true;

window.toggleSort = (field) => {
    if (sortField === field) {
        sortAsc = !sortAsc;
    } else {
        sortField = field;
        sortAsc = true;
    }
    updateAllSortIcons();
    currentPage = 1;
    renderObjects();
};

function updateAllSortIcons() {
    const activeArrow = sortAsc ? '\u25b2' : '\u25bc';
    const inactiveArrow = '\u21c5';
    
    ['key', 'size', 'date'].forEach(k => {
        const isActive = (sortField === k);
        
        // Update table header icons
        const thIcon = document.getElementById(`sort-icon-${k}`);
        if (thIcon) {
            thIcon.textContent = isActive ? activeArrow : inactiveArrow;
            thIcon.style.opacity = isActive ? '1' : '0.4';
        }
        
        // Update toolbar button icons + active state
        const btnIcon = document.getElementById(`sort-btn-icon-${k}`);
        if (btnIcon) {
            btnIcon.textContent = isActive ? activeArrow : inactiveArrow;
        }
        const btn = document.getElementById(`sort-btn-${k}`);
        if (btn) {
            if (isActive) {
                btn.classList.add('sort-active');
                btn.style.borderColor = 'var(--accent-primary)';
                btn.style.color = 'var(--accent-primary)';
            } else {
                btn.classList.remove('sort-active');
                btn.style.borderColor = '';
                btn.style.color = '';
            }
        }
    });
}

window.changePageSize = () => {
    currentPage = 1;
    renderObjects();
};

async function fetchObjects() {
    selectedObjects.clear();
    if(window.updateBulkActionsUI) updateBulkActionsUI();
    const tbody = document.getElementById('objects-table-body');
    tbody.innerHTML = '<tr><td colspan="4" style="text-align:center;"><div class="loading-spinner" style="margin:1rem auto;width:24px;height:24px;"></div></td></tr>';
    
    try {
        objectsCache = [];
        let marker = '';
        let hasMore = true;
        
        while(hasMore) {
            const url = `/_admin/buckets/${currentBucket}/objects?prefix=${encodeURIComponent(currentPrefix)}&delimiter=/&marker=${encodeURIComponent(marker)}&maxKeys=1000&_t=${Date.now()}`;
            const data = await api('GET', url);
            
            if (data.commonPrefixes) {
                data.commonPrefixes.forEach(p => {
                    objectsCache.push({ key: p, isFolder: true, size: 0, lastModified: '' });
                });
            }
            if (data.contents) {
                data.contents.forEach(obj => {
                    if (!obj.key.endsWith('/')) {
                        objectsCache.push({ key: obj.key, isFolder: false, size: obj.size, lastModified: obj.lastModified });
                    }
                });
            }
            
            hasMore = data.isTruncated;
            marker = data.nextMarker || '';
        }
        
        currentPage = 1;
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

    const sizeSelect = document.getElementById('page-size-select');
    const pageSize = sizeSelect ? parseInt(sizeSelect.value) : 10;
    const totalItems = filtered.length;
    const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
    
    if (currentPage > totalPages) currentPage = totalPages;
    if (currentPage < 1) currentPage = 1;
    
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = Math.min(startIndex + pageSize, totalItems);
    const paginated = filtered.slice(startIndex, endIndex);

    const paginationDiv = document.getElementById('objects-pagination');
    if (paginationDiv) {
        paginationDiv.style.display = totalItems > 0 ? 'flex' : 'none';
        document.getElementById('page-info').textContent = `Total: ${totalItems}`;
        
        const controls = document.getElementById('pagination-controls');
        let html = '';
        
        html += `<button class="btn btn-ghost" style="padding:0.2rem 0.5rem;" ${currentPage === 1 ? 'disabled' : ''} onclick="goToPage(${currentPage - 1})">&laquo; Prev</button>`;
        
        if (totalPages <= 7) {
            for(let i=1; i<=totalPages; i++) {
                html += `<button class="btn btn-ghost" style="padding:0.2rem 0.5rem; ${i === currentPage ? 'background:var(--accent-primary);color:white;' : ''}" onclick="goToPage(${i})">${i}</button>`;
            }
        } else {
            html += `<button class="btn btn-ghost" style="padding:0.2rem 0.5rem; ${1 === currentPage ? 'background:var(--accent-primary);color:white;' : ''}" onclick="goToPage(1)">1</button>`;
            
            let startPage = Math.max(2, currentPage - 2);
            let endPage = Math.min(totalPages - 1, currentPage + 2);
            
            if (currentPage <= 4) endPage = 5;
            else if (currentPage >= totalPages - 3) startPage = totalPages - 4;
            
            if (startPage > 2) html += `<span style="padding:0.2rem 0.5rem;color:var(--text-muted);">...</span>`;
            for(let i=startPage; i<=endPage; i++) {
                html += `<button class="btn btn-ghost" style="padding:0.2rem 0.5rem; ${i === currentPage ? 'background:var(--accent-primary);color:white;' : ''}" onclick="goToPage(${i})">${i}</button>`;
            }
            if (endPage < totalPages - 1) html += `<span style="padding:0.2rem 0.5rem;color:var(--text-muted);">...</span>`;
            
            html += `<button class="btn btn-ghost" style="padding:0.2rem 0.5rem; ${totalPages === currentPage ? 'background:var(--accent-primary);color:white;' : ''}" onclick="goToPage(${totalPages})">${totalPages}</button>`;
        }
        
        html += `<button class="btn btn-ghost" style="padding:0.2rem 0.5rem;" ${currentPage === totalPages ? 'disabled' : ''} onclick="goToPage(${currentPage + 1})">Next &raquo;</button>`;
        controls.innerHTML = html;
    }

    if (paginated.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--text-muted);">Empty — upload files or create a folder.</td></tr>';
        return;
    }

    tbody.innerHTML = paginated.map(obj => {
        const displayName = obj.isFolder ? obj.key.replace(currentPrefix, '').replace(/\/$/, '') : obj.key.replace(currentPrefix, '');
        if (obj.isFolder) {
            return `<tr>
                <td style="text-align:center;"><input type="checkbox" onclick="toggleObjectSelection(event, '${obj.key}')" ${selectedObjects.has(obj.key) ? 'checked' : ''}></td>
                <td onclick="navigatePrefix('${obj.key}')" style="cursor:pointer;"><div class="file-name"><svg viewBox="0 0 24 24" fill="none" stroke="var(--accent-primary)" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path></svg> ${displayName}/</div></td>
                <td onclick="navigatePrefix('${obj.key}')" style="cursor:pointer;">—</td><td onclick="navigatePrefix('${obj.key}')" style="cursor:pointer;">—</td><td></td></tr>`;
        }
        const lastMod = obj.lastModified ? new Date(obj.lastModified).toLocaleString() : '';
        const isImage = /\.(jpg|jpeg|png|gif|webp|svg)$/i.test(obj.key);
        const icon = isImage 
            ? `<svg viewBox="0 0 24 24" fill="none" stroke="var(--accent-primary)" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><circle cx="8.5" cy="8.5" r="1.5"></circle><polyline points="21 15 16 10 5 21"></polyline></svg>` 
            : `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path><polyline points="13 2 13 9 20 9"></polyline></svg>`;
        
        return `<tr>
            <td style="text-align:center;"><input type="checkbox" onclick="toggleObjectSelection(event, '${obj.key}')" ${selectedObjects.has(obj.key) ? 'checked' : ''}></td>
            <td><div class="file-name" ${isImage ? `onclick="previewObject('${obj.key}')" style="cursor:pointer;color:var(--accent-primary)"` : ''}>${icon} ${displayName}</div></td>
            <td>${formatBytes(obj.size)}</td>
            <td>${lastMod}</td>
            <td><div class="action-btns">
                <button class="btn btn-ghost" style="padding:0.4rem;" onclick="infoObject('${obj.key}')" title="Info"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg></button>
                <button class="btn btn-ghost" style="padding:0.4rem;" onclick="shareObject('${obj.key}')" title="Share Link"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="18" cy="5" r="3"></circle><circle cx="6" cy="12" r="3"></circle><circle cx="18" cy="19" r="3"></circle><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"></line><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"></line></svg></button>
                <button class="btn btn-ghost" style="padding:0.4rem;" onclick="downloadObject('${obj.key}')" title="Download"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg></button>
                <button class="btn btn-ghost" style="padding:0.4rem;" onclick="renameObject('${obj.key}')" title="Rename"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path></svg></button>
                <button class="btn btn-ghost text-danger" style="padding:0.4rem;" onclick="deleteObject('${obj.key}')" title="Delete"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg></button>
            </div></td></tr>`;
    }).join('');
}

window.createFolder = async () => {
    const name = prompt('Folder name:');
    if (!name) return;
    try {
        const key = currentPrefix + name + '/';
        const presignedData = await api('POST', '/_admin/presign', { method: 'PUT', bucket: currentBucket, key, expires: 3600 });
        const res = await fetch(presignedData.url, { method: 'PUT' });
        if (!res.ok) throw new Error('Failed to create folder');
        showToast(`Folder ${name} created`, 'success');
        fetchObjects();
    } catch (err) { showToast(err.message, 'error'); }
};

window.downloadObject = async (key) => {
    try {
        const data = await api('POST', '/_admin/presign', { method: 'GET', bucket: currentBucket, key, expires: 3600 });
        const a = document.createElement('a');
        a.href = data.url;
        const parts = key.split('/');
        a.download = parts[parts.length - 1] || 'download';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
    } catch (err) { showToast(err.message, 'error'); }
};

window.deleteObject = async (key) => {
    if (!confirm(`Delete ${key}?`)) return;
    try {
        const presignedData = await api('POST', '/_admin/presign', { method: 'DELETE', bucket: currentBucket, key, expires: 3600 });
        const res = await fetch(presignedData.url, { method: 'DELETE' });
        if (!res.ok) throw new Error('Failed to delete object');
        showToast('Deleted', 'success');
        fetchObjects();
    } catch (err) { showToast(err.message, 'error'); }
};

window.shareObject = async (key) => {
    try {
        // Presign for 7 days (604800 seconds)
        const data = await api('POST', '/_admin/presign', { method: 'GET', bucket: currentBucket, key, expires: 604800 });
        
        // Populate temporary presigned URL
        document.getElementById('share-url-input').value = data.url;
        
        // Extract base endpoint from presigned URL to build direct and virtual hosted URLs
        // (presignedData.url looks like http://localhost:9011/bucket/key?...)
        const urlObj = new URL(data.url);
        const host = urlObj.host; // e.g., localhost:9011 or s3.domain.com
        const protocol = urlObj.protocol;
        
        // 1. Direct URL (Path Style): http://host/bucket/key
        const directUrl = `${protocol}//${host}/${encodeURIComponent(currentBucket)}/${key.split('/').map(encodeURIComponent).join('/')}`;
        document.getElementById('share-direct-input').value = directUrl;
        
        // 2. Virtual-Hosted URL: http://bucket.host/key
        let vhost = host;
        if (host.startsWith('localhost') || host.startsWith('127.0.0.1')) {
            // For local development, vhost style requires editing /etc/hosts, so we just show the format
            vhost = `${currentBucket}.localhost` + (urlObj.port ? `:${urlObj.port}` : '');
        } else {
            vhost = `${currentBucket}.${host}`;
        }
        const vhostUrl = `${protocol}//${vhost}/${key.split('/').map(encodeURIComponent).join('/')}`;
        document.getElementById('share-vhost-input').value = vhostUrl;

        openModal('share-modal');
    } catch (err) { showToast(err.message, 'error'); }
};

window.renameObject = async (key) => {
    if (key.endsWith('/')) {
        showToast('Cannot rename folders directly. Please rename individual files.', 'error');
        return;
    }
    let oldName = key;
    if (key.startsWith(currentPrefix)) {
        oldName = key.substring(currentPrefix.length);
    }
    
    document.getElementById('rename-old-key').value = key;
    document.getElementById('rename-new-name').value = oldName;
    openModal('rename-modal');
};

window.executeRename = async () => {
    const key = document.getElementById('rename-old-key').value;
    const newName = document.getElementById('rename-new-name').value;
    
    let oldName = key;
    if (key.startsWith(currentPrefix)) {
        oldName = key.substring(currentPrefix.length);
    }

    if (!newName) {
        showToast('Rename cancelled', 'info');
        closeModal('rename-modal');
        return;
    }
    if (newName === oldName) {
        showToast('Name was not changed', 'info');
        closeModal('rename-modal');
        return;
    }
    
    closeModal('rename-modal');
    
    try {
        const newKey = currentPrefix + newName;
        await api('POST', '/_admin/rename', {
            bucket: currentBucket,
            oldKey: key,
            newKey: newKey,
        });
        showToast('Renamed successfully', 'success');
        fetchObjects();
    } catch (err) { 
        showToast(err.message || 'Rename failed', 'error'); 
        console.error(err);
    }
};

window.infoObject = async (key) => {
    try {
        const presign = await api('POST', '/_admin/presign', { method: 'HEAD', bucket: currentBucket, key: key, expires: 3600 });
        const res = await fetch(presign.url, { method: 'HEAD' });
        if (!res.ok) throw new Error('Failed to fetch object info');
        
        const size = res.headers.get('content-length') || '0';
        const type = res.headers.get('content-type') || 'Unknown';
        const etag = res.headers.get('etag') || 'N/A';
        let lastMod = res.headers.get('last-modified') || 'N/A';
        
        if (lastMod !== 'N/A') {
            lastMod = new Date(lastMod).toLocaleString();
        }
        
        document.getElementById('info-name').textContent = key;
        document.getElementById('info-size').textContent = formatBytes(parseInt(size));
        document.getElementById('info-type').textContent = type;
        document.getElementById('info-etag').textContent = etag;
        document.getElementById('info-date').textContent = lastMod;
        
        openModal('info-modal');
    } catch (err) { showToast(err.message, 'error'); }
};

window.previewObject = async (key) => {
    try {
        const title = document.getElementById('preview-title');
        const img = document.getElementById('preview-image');
        const loader = document.getElementById('preview-loading');
        
        title.textContent = key;
        img.style.display = 'none';
        loader.style.display = 'block';
        openModal('preview-modal');

        // Get short-lived URL for preview
        const data = await api('POST', '/_admin/presign', { method: 'GET', bucket: currentBucket, key, expires: 3600 });
        
        img.onload = () => {
            loader.style.display = 'none';
            img.style.display = 'block';
        };
        img.onerror = () => {
            loader.style.display = 'none';
            title.textContent = 'Preview failed';
        };
        img.src = data.url;
    } catch (err) { 
        closeModal('preview-modal');
        showToast(err.message, 'error'); 
    }
};

window.copyToClipboardInput = (id) => {
    const input = document.getElementById(id);
    if (input) {
        input.select();
        document.execCommand('copy');
        showToast('Copied to clipboard!', 'success');
    }
};


// Upload
const uploadZone = document.getElementById('upload-zone');
const fileInput = document.getElementById('file-input');
const folderInput = document.getElementById('folder-input');
if (uploadZone) {
    uploadZone.addEventListener('click', (e) => {
        if (e.target.tagName !== 'BUTTON') fileInput.click();
    });
    uploadZone.addEventListener('dragover', (e) => { e.preventDefault(); uploadZone.classList.add('dragover'); });
    uploadZone.addEventListener('dragleave', () => uploadZone.classList.remove('dragover'));
    uploadZone.addEventListener('drop', (e) => { 
        e.preventDefault(); 
        uploadZone.classList.remove('dragover'); 
        if (e.dataTransfer.files.length) handleUploads(e.dataTransfer.files); 
    });
}
if (fileInput) fileInput.addEventListener('change', () => { if (fileInput.files.length) handleUploads(fileInput.files); });
if (folderInput) folderInput.addEventListener('change', () => { if (folderInput.files.length) handleUploads(folderInput.files); });

async function handleUploads(files) {
    if (!currentBucket) { showToast('Select a bucket first', 'error'); return; }
    for (const file of files) {
        try {
            const relativePath = file.webkitRelativePath || file.name;
            showToast(`Uploading ${relativePath}...`, 'info');
            const key = `${currentPrefix}${relativePath}`;
            const data = await api('POST', '/_admin/presign', { method: 'PUT', bucket: currentBucket, key, expires: 3600 });
            const res = await fetch(data.url, { method: 'PUT', body: file });
            if (!res.ok) throw new Error(`Upload failed: ${res.status}`);
            showToast(`${relativePath} uploaded`, 'success');
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


// ==================== Multi-Select & Bulk Actions ====================
window.toggleObjectSelection = (e, key) => {
    e.stopPropagation();
    if (e.target.checked) selectedObjects.add(key);
    else selectedObjects.delete(key);
    updateBulkActionsUI();
    
    // Update header checkbox
    const allCheckboxes = document.querySelectorAll('#objects-table-body input[type="checkbox"]');
    const allChecked = Array.from(allCheckboxes).every(cb => cb.checked) && allCheckboxes.length > 0;
    const selectAllCb = document.getElementById("select-all-objects");
    if (selectAllCb) selectAllCb.checked = allChecked;
};

window.toggleSelectAllObjects = () => {
    const selectAllCb = document.getElementById("select-all-objects");
    const isChecked = selectAllCb ? selectAllCb.checked : false;
    const checkboxes = document.querySelectorAll('#objects-table-body input[type="checkbox"]');
    
    checkboxes.forEach(cb => {
        cb.checked = isChecked;
        const match = cb.getAttribute("onclick").match(/'([^']+)'/);
        if (match) {
            if (isChecked) selectedObjects.add(match[1]);
            else selectedObjects.delete(match[1]);
        }
    });
    updateBulkActionsUI();
};

window.updateBulkActionsUI = () => {
    const bulkActions = document.getElementById("bulk-actions");
    const selectedCountSpan = document.getElementById("selected-count");
    if (bulkActions && selectedCountSpan) {
        if (selectedObjects.size > 0) {
            bulkActions.style.display = "flex";
            selectedCountSpan.textContent = selectedObjects.size;
        } else {
            bulkActions.style.display = "none";
        }
    }
};

window.bulkDeleteObjects = async () => {
    if (selectedObjects.size === 0) return;
    if (!confirm(`Delete ${selectedObjects.size} selected items?`)) return;
    
    try {
        const keys = Array.from(selectedObjects);
        await api("POST", `/_admin/buckets/${currentBucket}/objects/delete`, keys);
        showToast(`Deleted ${keys.length} items`, "success");
        fetchObjects();
    } catch (err) {
        showToast(err.message, "error");
    }
};

window.bulkDownloadObjects = async () => {
    if (selectedObjects.size === 0) return;
    showToast(`Starting download for ${selectedObjects.size} items...`, "info");
    for (const key of selectedObjects) {
        try {
            const data = await api("POST", "/_admin/presign", { method: "GET", bucket: currentBucket, key, expires: 3600 });
            const a = document.createElement("a");
            a.href = data.url;
            a.download = key.split("/").pop() || "download";
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            await new Promise(r => setTimeout(r, 200));
        } catch (err) {
            showToast(`Failed to download ${key}: ${err.message}`, "error");
        }
    }
};

// ==================== Bucket Settings ====================
window.openBucketSettings = (name) => {
    currentBucket = name;
    document.getElementById('settings-bucket-name').innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="var(--accent-primary)" stroke-width="2" style="width:24px;height:24px;"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path></svg> <span>${name} Settings</span>`;
    
    // Update breadcrumb
    breadcrumb.innerHTML = `<span class="clickable" onclick="loadView('buckets')">Buckets</span><span class="separator">/</span><span style="color:var(--text-secondary)">${name} Settings</span>`;
    
    // Show view
    views.forEach(v => v.classList.remove('active'));
    document.getElementById('view-bucket-settings').classList.add('active');
    
    // Load overview tab by default
    switchBucketSettingsTab('overview');
};

window.switchBucketSettingsTab = (tab) => {
    ['overview', 'access', 'cors', 'lifecycle'].forEach(t => {
        const link = document.getElementById(`tab-link-${t}`);
        const content = document.getElementById(`tab-content-${t}`);
        if(link) link.classList.remove('active');
        if(content) content.style.display = 'none';
    });
    
    const activeLink = document.getElementById(`tab-link-${tab}`);
    const activeContent = document.getElementById(`tab-content-${tab}`);
    if(activeLink) activeLink.classList.add('active');
    if(activeContent) activeContent.style.display = 'block';
    
    if (tab === 'overview') {
        fetchBucketStats();
    } else if (tab === 'access') {
        fetchBucketPolicy();
    } else if (tab === 'cors') {
        fetchBucketCORS();
    } else if (tab === 'lifecycle') {
        fetchBucketLifecycle();
    }
};

async function fetchBucketStats() {
    const grid = document.getElementById('bucket-stats-grid');
    grid.innerHTML = '<div class="loading-spinner"></div>';
    try {
        const stats = await api('GET', `/_admin/buckets/${currentBucket}/stats`);
        grid.innerHTML = `
            <div class="glass-card info-card"><span class="info-label">Total Objects</span><span class="info-value" style="color:var(--accent-primary)">${stats.objects}</span></div>
            <div class="glass-card info-card"><span class="info-label">Storage Used</span><span class="info-value">${formatBytes(stats.bytes)}</span></div>
        `;
    } catch (err) {
        showToast(err.message, 'error');
        grid.innerHTML = '';
    }
}

async function fetchBucketPolicy() {
    try {
        const res = await api('GET', `/_admin/buckets/${currentBucket}/policy`);
        document.getElementById('bucket-policy-json').value = JSON.stringify(res, null, 2);
    } catch (err) {
        if (err.message.includes('not found')) {
            document.getElementById('bucket-policy-json').value = '';
        } else {
            showToast('Failed to fetch policy: ' + err.message, 'error');
        }
    }
}

window.validateJSONConfig = (textarea, errorDivId) => {
    const errorDiv = document.getElementById(errorDivId);
    if (!textarea || !errorDiv) return true;
    
    const val = textarea.value.trim();
    if (!val) {
        textarea.style.borderColor = 'rgba(255,255,255,0.1)';
        errorDiv.style.display = 'none';
        return true;
    }
    try {
        JSON.parse(val);
        textarea.style.borderColor = '#10b981'; // success green
        errorDiv.style.display = 'none';
        return true;
    } catch (e) {
        textarea.style.borderColor = '#ef4444'; // danger red
        errorDiv.textContent = 'Invalid JSON: ' + e.message;
        errorDiv.style.display = 'block';
        return false;
    }
};

window.setBucketPolicyPreset = (type) => {
    const policy = {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": "*",
                "Action": ["s3:GetObject"],
                "Resource": [`arn:aws:s3:::${currentBucket}/*`]
            }
        ]
    };
    const textarea = document.getElementById('bucket-policy-json');
    if (type === 'public') {
        textarea.value = JSON.stringify(policy, null, 2);
    } else {
        textarea.value = '';
    }
    validateJSONConfig(textarea, 'bucket-policy-error');
};

window.saveBucketPolicy = async () => {
    const textarea = document.getElementById('bucket-policy-json');
    const policyStr = textarea.value.trim();
    
    if (policyStr && !validateJSONConfig(textarea, 'bucket-policy-error')) {
        showToast('Please fix JSON errors before saving', 'error');
        return;
    }

    try {
        if (!policyStr) {
            await api('DELETE', `/_admin/buckets/${currentBucket}/policy`);
            showToast('Policy deleted (Bucket is private)', 'success');
        } else {
            const policyObj = JSON.parse(policyStr);
            await api('PUT', `/_admin/buckets/${currentBucket}/policy`, policyObj);
            showToast('Policy saved', 'success');
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
};

async function fetchBucketCORS() {
    try {
        const res = await api('GET', `/_admin/buckets/${currentBucket}/cors`);
        document.getElementById('bucket-cors-json').value = JSON.stringify(res, null, 2);
    } catch (err) {
        if (err.message.includes('not found') || err.message.includes('NoSuchCORSConfiguration') || err.message.includes('does not exist')) {
            document.getElementById('bucket-cors-json').value = '';
        } else {
            showToast('Failed to fetch CORS: ' + err.message, 'error');
        }
    }
}

window.setBucketCORSPreset = (type) => {
    const cors = {
        "CORSRules": [
            {
                "AllowedHeaders": ["*"],
                "AllowedMethods": ["GET", "PUT", "POST", "DELETE", "HEAD"],
                "AllowedOrigins": ["*"],
                "ExposeHeaders": ["ETag", "Content-Length", "x-amz-meta-custom-header"]
            }
        ]
    };
    const textarea = document.getElementById('bucket-cors-json');
    if (type === 'allow-all') {
        textarea.value = JSON.stringify(cors, null, 2);
    } else {
        textarea.value = '';
    }
    validateJSONConfig(textarea, 'bucket-cors-error');
};

window.saveBucketCORS = async () => {
    const textarea = document.getElementById('bucket-cors-json');
    const corsStr = textarea.value.trim();
    
    if (corsStr && !validateJSONConfig(textarea, 'bucket-cors-error')) {
        showToast('Please fix JSON errors before saving', 'error');
        return;
    }

    try {
        if (!corsStr) {
            await api('DELETE', `/_admin/buckets/${currentBucket}/cors`);
            showToast('CORS cleared', 'success');
        } else {
            const corsObj = JSON.parse(corsStr);
            await api('PUT', `/_admin/buckets/${currentBucket}/cors`, corsObj);
            showToast('CORS saved', 'success');
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
};


async function fetchBucketLifecycle() {
    try {
        const res = await api('GET', `/_admin/buckets/${currentBucket}/lifecycle`);
        document.getElementById('bucket-lifecycle-json').value = JSON.stringify(res, null, 2);
    } catch (err) {
        if (err.message.includes('not found')) {
            document.getElementById('bucket-lifecycle-json').value = '';
        } else {
            showToast('Failed to fetch lifecycle: ' + err.message, 'error');
        }
    }
}

window.setBucketLifecyclePreset = (type) => {
    const lifecycle = {
        "Rules": [
            {
                "ID": "ExpireAfter30Days",
                "Status": "Enabled",
                "Filter": {
                    "Prefix": ""
                },
                "Expiration": {
                    "Days": 30
                }
            }
        ]
    };
    const textarea = document.getElementById('bucket-lifecycle-json');
    if (type === 'expire-30d') {
        textarea.value = JSON.stringify(lifecycle, null, 2);
    } else {
        textarea.value = '';
    }
    validateJSONConfig(textarea, 'bucket-lifecycle-error');
};

window.saveBucketLifecycle = async () => {
    const textarea = document.getElementById('bucket-lifecycle-json');
    const lcStr = textarea.value.trim();
    
    if (lcStr && !validateJSONConfig(textarea, 'bucket-lifecycle-error')) {
        showToast('Please fix JSON errors before saving', 'error');
        return;
    }

    try {
        if (!lcStr) {
            await api('DELETE', `/_admin/buckets/${currentBucket}/lifecycle`);
            showToast('Lifecycle cleared', 'success');
        } else {
            const lcObj = JSON.parse(lcStr);
            await api('PUT', `/_admin/buckets/${currentBucket}/lifecycle`, lcObj);
            showToast('Lifecycle saved', 'success');
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
};

// ==================== Connection Info ====================

async function loadConnectionInfo() {
    const s3Host = window.location.hostname;
    const proto = window.location.protocol;
    // By default GoS3 S3 API runs on 9010
    const s3Port = '9010';
    
    document.getElementById('conn-endpoint').value = `${proto}//${s3Host}:${s3Port}`;
    
    try {
        const sas = await api('GET', '/api/v1/service-accounts');
        if (sas && sas.length > 0) {
            document.getElementById('conn-access').value = sas[0].accessKeyId;
        } else {
            document.getElementById('conn-access').value = '(No Access Key Found)';
        }
    } catch (e) {
        document.getElementById('conn-access').value = 'Error loading key';
    }
}

// ==================== Audit Logs ====================
function escapeHTML(str) {
    if (!str) return '';
    return str.toString().replace(/[&<>'"]/g, 
        tag => ({
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            "'": '&#39;',
            '"': '&quot;'
        }[tag] || tag)
    );
}

let globalAuditLogs = [];

async function loadAuditLogs() {
    const tbody = document.getElementById('audit-logs-table-body');
    if (!tbody) return;
    
    tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;">Loading...</td></tr>';
    try {
        globalAuditLogs = await api('GET', '/_admin/audit-logs') || [];
        
        // Reset filter
        const filterInput = document.getElementById('audit-log-filter');
        if (filterInput) filterInput.value = '';
        
        renderAuditLogs(globalAuditLogs);
    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="6" style="text-align:center; color:#ef4444;">Error: ${escapeHTML(err.message)}</td></tr>`;
        showToast('Failed to load audit logs', 'error');
    }
}

function renderAuditLogs(logs) {
    const tbody = document.getElementById('audit-logs-table-body');
    if (!tbody) return;

    if (!logs || logs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; color:var(--text-muted);">No audit logs found</td></tr>';
        return;
    }
    
    tbody.innerHTML = logs.map((log) => `
        <tr>
            <td style="white-space:nowrap; color:var(--text-secondary);">${new Date(log.timestamp).toLocaleString()}</td>
            <td><span class="badge" style="background:rgba(255,255,255,0.1); color:white;">${escapeHTML(log.user)}</span></td>
            <td><span style="color:#60a5fa; font-weight:500;">${escapeHTML(log.action)}</span></td>
            <td style="font-family:monospace; color:var(--text-primary);">${escapeHTML(log.target)}</td>
            <td style="color:var(--text-muted); font-family:monospace;">${escapeHTML(log.ip)}</td>
            <td>
                <button class="btn btn-ghost" style="padding:0.25rem 0.5rem; font-size:0.8rem;" onclick="viewAuditLogDetails('${escapeHTML(log.id)}')">Details</button>
            </td>
        </tr>
    `).join('');
}

function filterAuditLogs() {
    const query = (document.getElementById('audit-log-filter')?.value || '').toLowerCase();
    if (!query) {
        renderAuditLogs(globalAuditLogs);
        return;
    }
    const filtered = globalAuditLogs.filter(log => 
        (log.target && log.target.toLowerCase().includes(query)) ||
        (log.action && log.action.toLowerCase().includes(query)) ||
        (log.user && log.user.toLowerCase().includes(query))
    );
    renderAuditLogs(filtered);
}

function viewAuditLogDetails(id) {
    const log = globalAuditLogs.find(l => l.id === id);
    if (!log) return;
    
    const pre = document.getElementById('audit-log-details-content');
    if(pre) pre.textContent = JSON.stringify(log, null, 2);
    openModal('view-log-modal');
}

// ==================== IAM Policies ====================
async function fetchPolicies() {
    const tbody = document.getElementById('policies-table-body');
    if (!tbody) return;

    tbody.innerHTML = '<tr><td colspan="2" style="text-align:center;">Loading...</td></tr>';
    try {
        const policies = await api('GET', '/_admin/policies');
        renderPolicies(policies || []);
    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="2" style="text-align:center; color:#ef4444;">Error: ${escapeHTML(err.message)}</td></tr>`;
        showToast('Failed to load policies', 'error');
    }
}

function renderPolicies(policies) {
    const tbody = document.getElementById('policies-table-body');
    if (!tbody) return;

    if (!policies || policies.length === 0) {
        tbody.innerHTML = '<tr><td colspan="2" style="text-align:center; color:var(--text-muted);">No policies found</td></tr>';
        return;
    }

    tbody.innerHTML = policies.map((policy) => `
        <tr>
            <td style="font-weight:500; color:var(--text-primary); cursor:pointer;" onclick="editPolicy('${escapeHTML(policy)}')">${escapeHTML(policy)}</td>
            <td>
                <button class="btn btn-ghost" style="padding:0.25rem 0.5rem; font-size:0.8rem;" onclick="editPolicy('${escapeHTML(policy)}')">Edit</button>
                <button class="btn btn-ghost text-danger" style="padding:0.25rem 0.5rem; font-size:0.8rem;" onclick="deletePolicy('${escapeHTML(policy)}')">Delete</button>
            </td>
        </tr>
    `).join('');
}

function showCreatePolicyModal() {
    document.getElementById('policy-modal-title').textContent = 'Create Policy';
    document.getElementById('policy-name').value = '';
    document.getElementById('policy-name').readOnly = false;
    document.getElementById('policy-document').value = '';
    document.getElementById('policy-error').style.display = 'none';
    openModal('policy-modal');
}

async function editPolicy(name) {
    try {
        const policy = await api('GET', \`/_admin/policies/\${name}\`);
        document.getElementById('policy-modal-title').textContent = 'Edit Policy';
        document.getElementById('policy-name').value = name;
        document.getElementById('policy-name').readOnly = true;
        document.getElementById('policy-document').value = JSON.stringify(policy, null, 2);
        document.getElementById('policy-error').style.display = 'none';
        openModal('policy-modal');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function deletePolicy(name) {
    if (!confirm(\`Are you sure you want to delete policy "\${name}"?\`)) return;
    try {
        await api('DELETE', \`/_admin/policies/\${name}\`);
        showToast('Policy deleted successfully', 'success');
        fetchPolicies();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

document.getElementById('policy-form')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('policy-name').value.trim();
    const docStr = document.getElementById('policy-document').value.trim();

    try {
        const policy = JSON.parse(docStr);
        await api('PUT', \`/_admin/policies/\${name}\`, policy);
        closeModal('policy-modal');
        showToast('Policy saved successfully', 'success');
        fetchPolicies();
    } catch (err) {
        showToast(err.message, 'error');
    }
});
