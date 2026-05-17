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
    if (res.status === 401) {
        // Session expired or access key revoked — redirect to login
        currentUser = null;
        showLogin();
        showToast('Session expired. Please log in again.', 'error');
        throw new Error('Session expired');
    }
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

// Mobile Sidebar Toggle
const mobileMenuBtn = document.getElementById('mobile-menu-btn');
const mobileSidebarOverlay = document.getElementById('mobile-sidebar-overlay');
const sidebar = document.querySelector('.sidebar');

if (mobileMenuBtn) {
    mobileMenuBtn.addEventListener('click', () => {
        sidebar.classList.add('open');
        mobileSidebarOverlay.style.display = 'block';
    });
}

if (mobileSidebarOverlay) {
    mobileSidebarOverlay.addEventListener('click', () => {
        sidebar.classList.remove('open');
        mobileSidebarOverlay.style.display = 'none';
    });
}

// Navigation
navLinks.forEach(link => {
    link.addEventListener('click', () => {
        const view = link.dataset.view;
        navLinks.forEach(l => l.classList.remove('active'));
        link.classList.add('active');
        
        // Close mobile sidebar if open
        sidebar.classList.remove('open');
        if (mobileSidebarOverlay) mobileSidebarOverlay.style.display = 'none';
        
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
    } else if (viewName === 'settings') {
        breadcrumb.innerHTML = `<span>Global Settings</span>`;
        loadGlobalSettings();
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
        const statsHtml = typeof b.objects === 'number' ? `
            <div style="margin-top:0.5rem;font-size:0.85rem;color:var(--text-secondary);display:flex;gap:1rem;">
                <span><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:14px;height:14px;vertical-align:middle;margin-right:2px;"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="17 8 12 3 7 8"></polyline><line x1="12" y1="3" x2="12" y2="15"></line></svg> ${b.objects} objects</span>
                <span><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:14px;height:14px;vertical-align:middle;margin-right:2px;"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg> ${formatBytes(b.bytes)}</span>
            </div>
        ` : '';
        return `
            <div class="bucket-card" onclick="openBucket('${name}')">
                <div class="bucket-header">
                    <div class="bucket-icon">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path></svg>
                    </div>
                    <div class="bucket-info">
                        <h3>${name}</h3>
                        ${creationDate ? `<p>Created: ${creationDate}</p>` : ''}
                        ${statsHtml}
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
    topbarActions.innerHTML = `<button class="btn btn-primary" onclick="openModal('upload-staging-modal')"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="17 8 12 3 7 8"></polyline><line x1="12" y1="3" x2="12" y2="15"></line></svg> Upload</button>
    <button class="btn btn-ghost" onclick="openModal('create-folder-modal')"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path><line x1="12" y1="11" x2="12" y2="17"></line><line x1="9" y1="14" x2="15" y2="14"></line></svg> New Folder</button>`;
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
    const searchEl = document.getElementById('object-search');
    if (searchEl) searchEl.value = '';
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
        closeModal('create-bucket-modal');
        document.getElementById('create-bucket-form').reset();
        showToast(`Bucket ${name} created`, 'success');
        fetchBuckets();
    } catch (err) { showToast(err.message, 'error'); }
});

window.deleteBucket = async (name) => {
    const confirmation = prompt(`To confirm deletion of bucket '${name}', please type its name:`);
    if (confirmation !== name) {
        if (confirmation !== null) showToast('Bucket name does not match. Deletion cancelled.', 'error');
        return;
    }
    try {
        await api('DELETE', `/_admin/buckets/${name}`);
        showToast(`Bucket ${name} deleted`, 'success');
        fetchBuckets();
    } catch (err) {
        showToast(err.message, 'error');
    }
};

document.getElementById('create-folder-form')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('new-folder-name').value;
    try {
        const key = currentPrefix + name + '/';
        const presignedData = await api('POST', '/_admin/presign', { method: 'PUT', bucket: currentBucket, key, expires: 3600 });
        const res = await fetch(presignedData.url, { method: 'PUT' });
        if (!res.ok) throw new Error('Failed to create folder');
        
        closeModal('create-folder-modal');
        document.getElementById('create-folder-form').reset();
        showToast(`Folder ${name} created`, 'success');
        fetchObjects();
    } catch (err) { showToast(err.message, 'error'); }
});

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
    const activeArrow = sortAsc 
        ? `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12"><polyline points="18 15 12 9 6 15"></polyline></svg>` 
        : `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12"><polyline points="6 9 12 15 18 9"></polyline></svg>`;
    const inactiveArrow = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12"><polyline points="7 15 12 10 17 15"></polyline></svg>`;
    
    ['key', 'size', 'date'].forEach(k => {
        const isActive = (sortField === k);
        
        // Update table header icons
        const thIcon = document.getElementById(`sort-icon-${k}`);
        if (thIcon) {
            thIcon.innerHTML = isActive ? activeArrow : inactiveArrow;
            thIcon.style.opacity = isActive ? '1' : '0.4';
        }
        
        // Update toolbar button icons + active state
        const btnIcon = document.getElementById(`sort-btn-icon-${k}`);
        if (btnIcon) {
            btnIcon.innerHTML = isActive ? activeArrow : inactiveArrow;
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
            const safeKey = obj.key.replace(/'/g, "\\'");
            return `<tr>
                <td style="text-align:center;" data-label=""><input type="checkbox" onclick="toggleObjectSelection(event, '${safeKey}')" ${selectedObjects.has(obj.key) ? 'checked' : ''}></td>
                <td onclick="navigatePrefix('${safeKey}')" style="cursor:pointer;" data-label="Name"><div class="file-name"><svg viewBox="0 0 24 24" fill="none" stroke="var(--accent-primary)" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path></svg> ${displayName}/</div></td>
                <td onclick="navigatePrefix('${safeKey}')" style="cursor:pointer;" data-label="Size">—</td>
                <td onclick="navigatePrefix('${safeKey}')" style="cursor:pointer;" data-label="Last Modified">—</td>
                <td data-label="Actions"><div class="action-btns">
                    <button class="btn btn-ghost" style="padding:0.4rem;" onclick="infoFolder('${safeKey}')" title="Info"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg></button>
                    <button class="btn btn-ghost" style="padding:0.4rem;" onclick="downloadFolder('${safeKey}')" title="Download Folder (Zip)"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg></button>
                    <button class="btn btn-ghost text-danger" style="padding:0.4rem;" onclick="deleteObject('${safeKey}', true)" title="Delete Folder"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg></button>
                </div></td></tr>`;
        }
        const lastMod = obj.lastModified ? new Date(obj.lastModified).toLocaleString() : '';
        const isImage = /\.(jpg|jpeg|png|gif|webp|svg)$/i.test(obj.key);
        const icon = isImage 
            ? `<svg viewBox="0 0 24 24" fill="none" stroke="var(--accent-primary)" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><circle cx="8.5" cy="8.5" r="1.5"></circle><polyline points="21 15 16 10 5 21"></polyline></svg>` 
            : `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path><polyline points="13 2 13 9 20 9"></polyline></svg>`;
        
        const safeKey = obj.key.replace(/'/g, "\\'");
        return `<tr>
            <td style="text-align:center;" data-label=""><input type="checkbox" onclick="toggleObjectSelection(event, '${safeKey}')" ${selectedObjects.has(obj.key) ? 'checked' : ''}></td>
            <td data-label="Name"><div class="file-name" ${isImage ? `onclick="previewObject('${safeKey}')" style="cursor:pointer;color:var(--accent-primary)"` : ''}>${icon} ${displayName}</div></td>
            <td data-label="Size">${formatBytes(obj.size)}</td>
            <td data-label="Last Modified">${lastMod}</td>
            <td data-label="Actions"><div class="action-btns">
                <button class="btn btn-ghost" style="padding:0.4rem;" onclick="infoObject('${safeKey}')" title="Info"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg></button>
                <button class="btn btn-ghost" style="padding:0.4rem;" onclick="shareObject('${safeKey}')" title="Share Link"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="18" cy="5" r="3"></circle><circle cx="6" cy="12" r="3"></circle><circle cx="18" cy="19" r="3"></circle><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"></line><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"></line></svg></button>
                <button class="btn btn-ghost" style="padding:0.4rem;" onclick="downloadObject('${safeKey}')" title="Download"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg></button>
                <button class="btn btn-ghost" style="padding:0.4rem;" onclick="renameObject('${safeKey}')" title="Rename"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path></svg></button>
                <button class="btn btn-ghost text-danger" style="padding:0.4rem;" onclick="deleteObject('${safeKey}')" title="Delete"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg></button>
            </div></td></tr>`;
    }).join('');
}

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

window.downloadFolder = async (key) => {
    try {
        showToast('Preparing folder download...', 'info');
        const url = `/_admin/buckets/${currentBucket}/download-folder?prefix=${encodeURIComponent(key)}`;
        const a = document.createElement('a');
        a.href = url;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
    } catch (err) { showToast(err.message, 'error'); }
};

window.infoFolder = async (key) => {
    try {
        let totalSize = 0;
        let fileCount = 0;
        let lastMod = null;
        let marker = '';
        let hasMore = true;
        
        while(hasMore) {
            const url = `/_admin/buckets/${currentBucket}/objects?prefix=${encodeURIComponent(key)}&marker=${encodeURIComponent(marker)}&maxKeys=1000`;
            const data = await api('GET', url);
            if (data.contents) {
                data.contents.forEach(obj => {
                    if (obj.key !== key) {
                        totalSize += obj.size;
                        fileCount++;
                        const objDate = new Date(obj.lastModified);
                        if (!lastMod || objDate > lastMod) lastMod = objDate;
                    }
                });
            }
            hasMore = data.isTruncated;
            marker = data.nextMarker || '';
        }
        
        document.getElementById('info-name').textContent = key;
        document.getElementById('info-size').textContent = `${formatBytes(totalSize)} (${fileCount} files)`;
        document.getElementById('info-type').textContent = 'Folder';
        document.getElementById('info-etag').textContent = 'N/A';
        document.getElementById('info-date').textContent = lastMod ? lastMod.toLocaleString() : 'N/A';
        
        openModal('info-modal');
    } catch (err) { showToast(err.message, 'error'); }
};

window.deleteObject = async (key, isFolder = false) => {
    const itemName = isFolder ? key.replace(/\/$/, '') : key;
    const itemType = isFolder ? 'folder' : 'file';
    const shortName = itemName.split('/').pop();
    
    const confirmation = prompt(`To confirm deletion of ${itemType} '${shortName}', please type its name:`);
    if (confirmation !== shortName) {
        if (confirmation !== null) showToast('Name does not match. Deletion cancelled.', 'error');
        return;
    }
    
    try {
        if (isFolder) {
            let marker = '';
            let hasMore = true;
            let keysToDelete = [];
            
            while(hasMore) {
                const url = `/_admin/buckets/${currentBucket}/objects?prefix=${encodeURIComponent(key)}&marker=${encodeURIComponent(marker)}&maxKeys=1000`;
                const data = await api('GET', url);
                if (data.contents) {
                    data.contents.forEach(obj => keysToDelete.push(obj.key));
                }
                hasMore = data.isTruncated;
                marker = data.nextMarker || '';
            }
            if (!keysToDelete.includes(key)) keysToDelete.push(key);
            
            for (let i = 0; i < keysToDelete.length; i += 1000) {
                const batch = keysToDelete.slice(i, i + 1000);
                await api('POST', `/_admin/buckets/${currentBucket}/objects/delete`, batch);
            }
            showToast(`Deleted folder and ${keysToDelete.length} items inside`, 'success');
        } else {
            const presignedData = await api('POST', '/_admin/presign', { method: 'DELETE', bucket: currentBucket, key, expires: 3600 });
            const res = await fetch(presignedData.url, { method: 'DELETE' });
            if (!res.ok) throw new Error('Failed to delete object');
            showToast('Deleted', 'success');
        }
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
        const video = document.getElementById('preview-video');
        const audio = document.getElementById('preview-audio');
        const text = document.getElementById('preview-text');
        const loader = document.getElementById('preview-loading');
        
        title.textContent = key;
        img.style.display = 'none';
        if (video) video.style.display = 'none';
        if (audio) audio.style.display = 'none';
        if (text) text.style.display = 'none';
        
        // Pause any playing media
        if (video) { video.pause(); video.removeAttribute('src'); video.load(); }
        if (audio) { audio.pause(); audio.removeAttribute('src'); audio.load(); }
        
        loader.style.display = 'block';
        openModal('preview-modal');

        // Get short-lived URL for preview
        const data = await api('POST', '/_admin/presign', { method: 'GET', bucket: currentBucket, key, expires: 3600 });
        
        const ext = key.split('.').pop().toLowerCase();
        const imageExts = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico'];
        const videoExts = ['mp4', 'webm', 'ogg', 'mov'];
        const audioExts = ['mp3', 'wav', 'ogg', 'aac', 'flac'];
        const textExts = ['txt', 'md', 'csv', 'json', 'xml', 'js', 'html', 'css', 'go', 'py', 'sh', 'yaml', 'yml'];
        
        if (imageExts.includes(ext)) {
            img.onload = () => {
                loader.style.display = 'none';
                img.style.display = 'block';
            };
            img.onerror = () => {
                loader.style.display = 'none';
                title.textContent = 'Preview failed';
            };
            img.src = data.url;
        } else if (videoExts.includes(ext)) {
            loader.style.display = 'none';
            video.style.display = 'block';
            video.src = data.url;
            video.play().catch(e => console.error("Auto-play prevented", e));
        } else if (audioExts.includes(ext)) {
            loader.style.display = 'none';
            audio.style.display = 'block';
            audio.src = data.url;
            audio.play().catch(e => console.error("Auto-play prevented", e));
        } else if (textExts.includes(ext)) {
            try {
                const response = await fetch(data.url);
                const textContent = await response.text();
                loader.style.display = 'none';
                text.style.display = 'block';
                text.textContent = textContent;
                text.style.textAlign = 'left';
                text.style.color = 'var(--text-primary)';
            } catch (e) {
                loader.style.display = 'none';
                title.textContent = 'Failed to load text content';
            }
        } else {
            // Fallback for unsupported types
            loader.style.display = 'none';
            text.style.display = 'block';
            text.textContent = 'Preview not supported for this file type.\nPlease download the file to view its contents.';
            text.style.textAlign = 'center';
            text.style.color = 'var(--text-secondary)';
        }
    } catch (err) { 
        closeModal('preview-modal');
        showToast(err.message, 'error'); 
    }
};

// Also pause media when modal closes
const originalCloseModal = window.closeModal;
window.closeModal = (id) => {
    if (id === 'preview-modal') {
        const video = document.getElementById('preview-video');
        const audio = document.getElementById('preview-audio');
        if (video) { video.pause(); video.removeAttribute('src'); video.load(); }
        if (audio) { audio.pause(); audio.removeAttribute('src'); audio.load(); }
    }
    if (originalCloseModal) originalCloseModal(id);
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
// File Manager v2 - Upload Staging Queue
let uploadStagingQueue = [];
const uploadZone = document.getElementById('upload-zone');
const stagingFileInput = document.getElementById('staging-file-input');
const stagingFolderInput = document.getElementById('staging-folder-input');

if (uploadZone) {
    uploadZone.addEventListener('click', (e) => {
        if (e.target.tagName !== 'BUTTON') {
            openModal('upload-staging-modal');
        }
    });
    uploadZone.addEventListener('dragover', (e) => { e.preventDefault(); uploadZone.classList.add('dragover'); });
    uploadZone.addEventListener('dragleave', () => uploadZone.classList.remove('dragover'));
    uploadZone.addEventListener('drop', (e) => { 
        e.preventDefault(); 
        uploadZone.classList.remove('dragover'); 
        if (e.dataTransfer.files.length) {
            stageFiles(e.dataTransfer.files);
            openModal('upload-staging-modal');
        }
    });
}

if (stagingFileInput) {
    stagingFileInput.addEventListener('change', () => { 
        if (stagingFileInput.files.length) stageFiles(stagingFileInput.files); 
        stagingFileInput.value = ''; // Reset
    });
}
if (stagingFolderInput) {
    stagingFolderInput.addEventListener('change', () => { 
        if (stagingFolderInput.files.length) stageFiles(stagingFolderInput.files); 
        stagingFolderInput.value = ''; // Reset
    });
}

// Intercept old buttons in upload-zone if they exist
window.triggerStagingInput = (type) => {
    if (type === 'file') stagingFileInput.click();
    if (type === 'folder') stagingFolderInput.click();
    openModal('upload-staging-modal');
};

function stageFiles(files) {
    for (let i = 0; i < files.length; i++) {
        uploadStagingQueue.push(files[i]);
    }
    renderStagingQueue();
}

window.removeStagingItem = (index) => {
    uploadStagingQueue.splice(index, 1);
    renderStagingQueue();
};

window.clearStagingQueue = () => {
    uploadStagingQueue = [];
    renderStagingQueue();
    document.getElementById('staging-status').textContent = '';
};

function renderStagingQueue() {
    const tbody = document.getElementById('staging-table-body');
    const status = document.getElementById('staging-status');
    if (!tbody) return;

    if (uploadStagingQueue.length === 0) {
        tbody.innerHTML = '<tr><td colspan="3" style="text-align:center; color:var(--text-muted); padding: 2rem;">Queue is empty. Add files or folders to begin.</td></tr>';
        if (status) status.textContent = '';
        return;
    }

    let totalSize = 0;
    tbody.innerHTML = uploadStagingQueue.map((file, index) => {
        totalSize += file.size;
        const relativePath = file.webkitRelativePath || file.name;
        return `
            <tr>
                <td style="word-break: break-all;"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:16px;height:16px;margin-right:8px;vertical-align:middle;color:var(--text-muted);"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path><polyline points="13 2 13 9 20 9"></polyline></svg>${escapeHTML(relativePath)}</td>
                <td>${formatBytes(file.size)}</td>
                <td style="text-align:right;">
                    <button class="btn btn-ghost text-danger btn-sm" onclick="removeStagingItem(${index})"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg></button>
                </td>
            </tr>
        `;
    }).join('');

    if (status) {
        status.textContent = `${uploadStagingQueue.length} files (${formatBytes(totalSize)})`;
    }
}

window.startStagingUpload = async () => {
    if (!currentBucket) { showToast('Select a bucket first', 'error'); return; }
    if (uploadStagingQueue.length === 0) return;

    const btnStart = document.getElementById('btn-start-staging-upload');
    const status = document.getElementById('staging-status');
    const originalText = btnStart.textContent;
    btnStart.disabled = true;
    btnStart.textContent = 'Uploading...';

    let successCount = 0;
    let failCount = 0;

    for (let i = 0; i < uploadStagingQueue.length; i++) {
        const file = uploadStagingQueue[i];
        try {
            const relativePath = file.webkitRelativePath || file.name;
            status.textContent = `Uploading ${i+1}/${uploadStagingQueue.length}...`;
            const key = `${currentPrefix}${relativePath}`;
            const data = await api('POST', '/_admin/presign', { method: 'PUT', bucket: currentBucket, key, expires: 3600 });
            const res = await fetch(data.url, { method: 'PUT', body: file });
            if (!res.ok) throw new Error(`HTTP ${res.status}`);
            successCount++;
        } catch (err) { 
            console.error(err);
            failCount++;
        }
    }

    if (failCount === 0) {
        showToast(`Successfully uploaded ${successCount} files`, 'success');
        clearStagingQueue();
        closeModal('upload-staging-modal');
    } else {
        showToast(`Uploaded ${successCount} files, failed ${failCount}`, 'error');
        // We could theoretically remove the successful ones from the queue, 
        // but for simplicity we'll just let the user see the error and clear manually if they want.
    }

    btnStart.disabled = false;
    btnStart.textContent = originalText;
    fetchObjects();
};


// ==================== Service Accounts ====================
async function fetchServiceAccounts() {
    const tbody = document.getElementById('sa-table-body');
    if (!tbody) return;
    tbody.innerHTML = '<tr><td colspan="4"><div class="loading-spinner"></div></td></tr>';
    try {
        const accounts = await api('GET', '/api/v1/service-accounts');
        tbody.innerHTML = accounts.map(sa => `
            <tr>
                <td data-label="Access Key"><code>${sa.accessKeyId}</code></td>
                <td data-label="Description">${sa.description || '—'}</td>
                <td data-label="Created">${new Date(sa.createdAt).toLocaleString()}</td>
                <td data-label="Actions"><button class="btn btn-ghost text-danger" onclick="deleteServiceAccount('${sa.id}')">Delete</button></td>
            </tr>`).join('');
    } catch (err) { showToast(err.message, 'error'); tbody.innerHTML = ''; }
}

let globalS3Endpoint = null;
async function getS3Endpoint() {
    if (globalS3Endpoint) return globalS3Endpoint;
    try {
        const info = await api('GET', '/_admin/info');
        if (info.config && info.config.base_domain) {
            globalS3Endpoint = (info.config.tls_enabled ? 'https://' : 'http://') + info.config.base_domain;
            if (info.config.port !== 80 && info.config.port !== 443) {
                globalS3Endpoint += ':' + info.config.port;
            }
        } else {
            const port = info.config?.port || 9000;
            globalS3Endpoint = window.location.protocol + '//' + window.location.hostname + ':' + port;
        }
    } catch (e) {
        globalS3Endpoint = window.location.protocol + '//' + window.location.hostname + ':9000';
    }
    return globalS3Endpoint;
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
            document.getElementById('sa-created-endpoint').textContent = await getS3Endpoint();
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
                <td data-label="Username"><strong>${u.username}</strong>${u.isRoot ? ' <span class="text-accent" style="font-size:0.8rem;background:rgba(0,240,255,0.1);padding:2px 6px;border-radius:4px;margin-left:8px;">ROOT</span>' : ''}</td>
                <td data-label="Status">${u.disabled ? '<span style="color:var(--danger)">Disabled</span>' : '<span style="color:var(--success)">Active</span>'}</td>
                <td data-label="Actions">${!u.isRoot ? `<button class="btn btn-ghost text-danger" onclick="deleteUser('${u.username}')">Delete</button>` : ''}</td>
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
        const endpoint = await getS3Endpoint();
        const ramPct = info.system?.ram_sys ? Math.round((info.system.ram_alloc / info.system.ram_sys) * 100) : 0;
        const diskPct = info.storage?.disk_total ? Math.round(((info.storage.disk_total - info.storage.disk_free) / info.storage.disk_total) * 100) : 0;
        
        grid.innerHTML = `
            <div class="glass-card info-card"><span class="info-label">Version</span><span class="info-value" style="color:var(--accent-primary)">${info.version || '-'}</span></div>
            <div class="glass-card info-card"><span class="info-label">Uptime</span><span class="info-value">${formatDuration(info.uptime_seconds || 0)}</span></div>
            
            <div class="glass-card info-card"><span class="info-label">Resources</span>
                <div style="font-size:0.9rem; margin-top:0.5rem; display:flex; flex-direction:column; gap:0.25rem;">
                    <div style="display:flex; justify-content:space-between;"><span>Total Buckets</span><b>${info.storage?.buckets || 0}</b></div>
                    <div style="display:flex; justify-content:space-between;"><span>Total Objects</span><b>${info.storage?.total_objects || 0}</b></div>
                    <div style="display:flex; justify-content:space-between;"><span>IAM Users</span><b>${info.system?.users_count || 0}</b></div>
                </div>
            </div>
            
            <div class="glass-card info-card"><span class="info-label">Hardware</span>
                <div style="font-size:0.9rem; margin-top:0.5rem; display:flex; flex-direction:column; gap:0.25rem;">
                    <div style="display:flex; justify-content:space-between;"><span>CPU Cores</span><b>${info.system?.cpu_cores || 0}</b></div>
                    <div style="display:flex; justify-content:space-between;"><span>Goroutines</span><b>${info.system?.goroutines || 0}</b></div>
                </div>
            </div>
            
            <div class="glass-card info-card"><span class="info-label">RAM Usage</span>
                <div style="margin-top:0.8rem; background:rgba(255,255,255,0.1); border-radius:4px; height:8px; overflow:hidden;">
                    <div style="width:${ramPct}%; background:var(--accent-secondary); height:100%;"></div>
                </div>
                <div style="font-size:0.8rem; margin-top:0.4rem; display:flex; justify-content:space-between; color:var(--text-muted);">
                    <span>${formatBytes(info.system?.ram_alloc || 0)}</span>
                    <span>${formatBytes(info.system?.ram_sys || 0)} Total</span>
                </div>
            </div>
            
            <div class="glass-card info-card"><span class="info-label">Disk Storage</span>
                <div style="margin-top:0.8rem; background:rgba(255,255,255,0.1); border-radius:4px; height:8px; overflow:hidden;">
                    <div style="width:${diskPct}%; background:${diskPct > 80 ? 'var(--danger)' : 'var(--success)'}; height:100%;"></div>
                </div>
                <div style="font-size:0.8rem; margin-top:0.4rem; display:flex; justify-content:space-between; color:var(--text-muted);">
                    <span>${formatBytes((info.storage?.disk_total || 0) - (info.storage?.disk_free || 0))}</span>
                    <span>${formatBytes(info.storage?.disk_total || 0)} Total</span>
                </div>
            </div>
            
            <div class="glass-card info-card" style="grid-column:1/-1"><span class="info-label">Configuration</span>
                <div style="font-size:0.9rem; margin-top:0.8rem; display:flex; flex-direction:column; gap:0.5rem;">
                    <div style="display:flex; justify-content:space-between;"><span>Max Object Size</span><b>${formatBytes(info.config?.max_size || 0)}</b></div>
                    <div style="display:flex; justify-content:space-between;"><span>Server Port</span><b>${info.config?.port || 9000}</b></div>
                    <div style="display:flex; justify-content:space-between; word-break:break-all;"><span>Data Dir</span><b style="font-family:monospace; margin-left:1rem;">${info.config?.data_dir || '-'}</b></div>
                    <div style="display:flex; justify-content:space-between; word-break:break-all;"><span>API Endpoint</span><b style="font-family:monospace; margin-left:1rem;">${endpoint}</b></div>
                </div>
            </div>`;
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
    ['overview', 'access', 'versioning', 'cors', 'lifecycle', 'website', 'domains', 'webhooks'].forEach(t => {
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
    } else if (tab === 'versioning') {
        fetchBucketVersioning();
    } else if (tab === 'cors') {
        fetchBucketCORS();
    } else if (tab === 'lifecycle') {
        fetchBucketLifecycle();
    } else if (tab === 'website') {
        fetchBucketWebsite();
    } else if (tab === 'domains') {
        fetchBucketCustomDomains();
    } else if (tab === 'webhooks') {
        fetchBucketWebhooks();
    }
};

window.fetchBucketVersioning = async () => {
    try {
        const res = await api('GET', `/_admin/buckets/${currentBucket}/versioning`);
        const isEnabled = res.status === "Enabled";
        const isSuspended = res.status === "Suspended";
        
        document.getElementById('bucket-versioning-toggle').checked = isEnabled;
        
        const label = document.getElementById('versioning-status-label');
        if (isEnabled) {
            label.textContent = "Enabled";
            label.style.background = "rgba(100,255,100,0.1)";
            label.style.color = "#64ff64";
            label.style.border = "1px solid rgba(100,255,100,0.3)";
        } else if (isSuspended) {
            label.textContent = "Suspended";
            label.style.background = "rgba(255,200,50,0.1)";
            label.style.color = "#ffc832";
            label.style.border = "1px solid rgba(255,200,50,0.3)";
        } else {
            label.textContent = "Disabled";
            label.style.background = "var(--bg-secondary)";
            label.style.color = "var(--text-secondary)";
            label.style.border = "1px solid var(--border-color)";
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
};

window.toggleBucketVersioning = async (checkbox) => {
    const status = checkbox.checked ? "Enabled" : "Suspended";
    try {
        await api('PUT', `/_admin/buckets/${currentBucket}/versioning`, { status });
        showToast(`Bucket versioning is now ${status}`, 'success');
        fetchBucketVersioning();
    } catch (err) {
        showToast(err.message, 'error');
        checkbox.checked = !checkbox.checked; // Revert
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
    await loadServiceAccountsForBucketPolicy();
}

async function loadServiceAccountsForBucketPolicy() {
    const select = document.getElementById('policy-sa-select');
    if (!select) return;
    try {
        const sas = await api('GET', '/_admin/service-accounts');
        select.innerHTML = '<option value="">Select Service Account...</option>';
        if (sas && sas.length > 0) {
            sas.forEach(sa => {
                const opt = document.createElement('option');
                opt.value = sa.accessKeyId;
                opt.textContent = `${sa.description || 'No Description'} (${sa.accessKeyId})`;
                select.appendChild(opt);
            });
        }
    } catch (err) {
        console.error('Failed to load service accounts for policy UI', err);
    }
}

window.addServiceAccountToPolicy = () => {
    const accessKey = document.getElementById('policy-sa-select').value;
    const permission = document.getElementById('policy-sa-permission').value;
    if (!accessKey) {
        showToast('Please select a service account', 'error');
        return;
    }

    const textarea = document.getElementById('bucket-policy-json');
    let policyObj = {
        "Version": "2012-10-17",
        "Statement": []
    };

    if (textarea.value.trim()) {
        try {
            policyObj = JSON.parse(textarea.value.trim());
        } catch (e) {
            showToast('Cannot add: Current JSON is invalid', 'error');
            return;
        }
    }

    if (!policyObj.Statement) policyObj.Statement = [];

    const saArn = `arn:aws:iam:::serviceaccount/${accessKey}`;
    let actions = ["s3:GetObject", "s3:ListBucket"];
    if (permission === 'readwrite') {
        actions = ["s3:*"];
    }

    policyObj.Statement.push({
        "Effect": "Allow",
        "Principal": { "AWS": saArn },
        "Action": actions,
        "Resource": [
            `arn:aws:s3:::${currentBucket}`,
            `arn:aws:s3:::${currentBucket}/*`
        ]
    });

    textarea.value = JSON.stringify(policyObj, null, 2);
    validateJSONConfig(textarea, 'bucket-policy-error');
    showToast('Policy updated. Click Save Policy to apply.', 'success');
};

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


async function fetchBucketWebsite() {
    try {
        const res = await api('GET', `/_admin/buckets/${currentBucket}/website`);
        document.getElementById('bucket-website-json').value = JSON.stringify(res, null, 2);
    } catch (err) {
        if (err.message.includes('not found')) {
            document.getElementById('bucket-website-json').value = '';
        } else {
            showToast('Failed to fetch website config: ' + err.message, 'error');
        }
    }
}

window.setBucketWebsitePreset = (type) => {
    const website = {
        "IndexDocument": {
            "Suffix": "index.html"
        },
        "ErrorDocument": {
            "Key": "error.html"
        }
    };
    const textarea = document.getElementById('bucket-website-json');
    if (type === 'default') {
        textarea.value = JSON.stringify(website, null, 2);
    } else {
        textarea.value = '';
    }
    validateJSONConfig(textarea, 'bucket-website-error');
};

window.saveBucketWebsite = async () => {
    const textarea = document.getElementById('bucket-website-json');
    const wsStr = textarea.value.trim();
    
    if (wsStr && !validateJSONConfig(textarea, 'bucket-website-error')) {
        showToast('Please fix JSON errors before saving', 'error');
        return;
    }

    try {
        if (!wsStr) {
            await api('DELETE', `/_admin/buckets/${currentBucket}/website`);
            showToast('Website config cleared', 'success');
        } else {
            const wsObj = JSON.parse(wsStr);
            await api('PUT', `/_admin/buckets/${currentBucket}/website`, wsObj);
            showToast('Website config saved', 'success');
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
};


async function fetchBucketCustomDomains() {
    try {
        const domains = await api('GET', `/_admin/buckets/${currentBucket}/domains`);
        renderBucketCustomDomains(domains || []);
    } catch (err) {
        showToast('Failed to fetch custom domains: ' + err.message, 'error');
    }
}

function renderBucketCustomDomains(domains) {
    const list = document.getElementById('bucket-domains-list');
    if (domains.length === 0) {
        list.innerHTML = `<div style="color:var(--text-secondary);font-style:italic;padding:0.5rem 0;">No custom domains mapped to this bucket.</div>`;
        return;
    }
    
    list.innerHTML = domains.map(d => `
        <div style="display:flex;justify-content:space-between;align-items:center;background:var(--bg-tertiary);padding:0.75rem 1rem;border-radius:4px;border:1px solid rgba(255,255,255,0.05);">
            <div style="display:flex;align-items:center;gap:0.5rem;">
                <svg viewBox="0 0 24 24" fill="none" stroke="var(--accent-primary)" stroke-width="2" style="width:16px;height:16px;"><circle cx="12" cy="12" r="10"></circle><line x1="2" y1="12" x2="22" y2="12"></line><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path></svg>
                <a href="http://${d}" target="_blank" style="color:var(--text-primary);text-decoration:none;">${d}</a>
            </div>
            <button class="btn btn-ghost text-danger btn-sm" onclick="deleteBucketCustomDomain('${d}')" style="padding:0.25rem 0.5rem;">Remove</button>
        </div>
    `).join('');
}

window.addBucketCustomDomain = async () => {
    const input = document.getElementById('new-custom-domain');
    const domain = input.value.trim();
    if (!domain) return;
    
    try {
        await api('PUT', `/_admin/buckets/${currentBucket}/domains`, { domain });
        input.value = '';
        showToast('Domain mapped successfully', 'success');
        fetchBucketCustomDomains();
    } catch (err) {
        showToast(err.message, 'error');
    }
};

window.deleteBucketCustomDomain = async (domain) => {
    if (!confirm(`Are you sure you want to remove the domain mapping for ${domain}?`)) return;
    
    try {
        await api('DELETE', `/_admin/buckets/${currentBucket}/domains/${encodeURIComponent(domain)}`);
        showToast('Domain mapping removed', 'success');
        fetchBucketCustomDomains();
    } catch (err) {
        showToast(err.message, 'error');
    }
};


async function fetchBucketWebhooks() {
    try {
        const config = await api('GET', `/_admin/buckets/${currentBucket}/notifications`);
        const textarea = document.getElementById('bucket-webhooks-json');
        if (!config || !config.webhooks || config.webhooks.length === 0) {
            textarea.value = '';
        } else {
            textarea.value = JSON.stringify(config, null, 2);
        }
    } catch (err) {
        showToast('Failed to fetch webhooks: ' + err.message, 'error');
    }
}

window.addBucketWebhookTemplate = () => {
    const template = {
        webhooks: [
            {
                events: ["s3:ObjectCreated:*", "s3:ObjectRemoved:*"],
                url: "https://your-api.com/webhook",
                secret: "your_optional_hmac_secret"
            }
        ]
    };
    document.getElementById('bucket-webhooks-json').value = JSON.stringify(template, null, 2);
    document.getElementById('bucket-webhooks-error').style.display = 'none';
};

window.saveBucketWebhooks = async () => {
    const textarea = document.getElementById('bucket-webhooks-json');
    const val = textarea.value.trim();
    const errorEl = document.getElementById('bucket-webhooks-error');
    
    if (!val) {
        try {
            await api('DELETE', `/_admin/buckets/${currentBucket}/notifications`);
            showToast('Webhooks deleted successfully', 'success');
        } catch (err) {
            showToast(err.message, 'error');
        }
        return;
    }

    try {
        const payload = JSON.parse(val);
        await api('PUT', `/_admin/buckets/${currentBucket}/notifications`, payload);
        showToast('Webhooks saved successfully', 'success');
    } catch (err) {
        errorEl.textContent = 'Invalid JSON: ' + err.message;
        errorEl.style.display = 'block';
    }
};

// ==================== Connection Info ====================




async function loadConnectionInfo() {
    const endpoint = await getS3Endpoint();
    document.getElementById('conn-endpoint').value = endpoint;
    
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
            <td data-label="Timestamp" style="white-space:nowrap; color:var(--text-secondary);">${new Date(log.timestamp).toLocaleString()}</td>
            <td data-label="User"><span class="badge" style="background:rgba(255,255,255,0.1); color:white;">${escapeHTML(log.user)}</span></td>
            <td data-label="Action"><span style="color:#60a5fa; font-weight:500;">${escapeHTML(log.action)}</span></td>
            <td data-label="Target" style="font-family:monospace; color:var(--text-primary);">${escapeHTML(log.target)}</td>
            <td data-label="IP Address" style="color:var(--text-muted); font-family:monospace;">${escapeHTML(log.ip)}</td>
            <td data-label="Actions">
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
            <td data-label="Policy Name" style="font-weight:500; color:var(--text-primary); cursor:pointer;" onclick="editPolicy('${escapeHTML(policy)}')">${escapeHTML(policy)}</td>
            <td data-label="Actions">
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
    loadPolicyTemplateSelect();
    openModal('policy-modal');
}

let policyTemplatesCache = null;

async function loadPolicyTemplateSelect() {
    const select = document.getElementById('policy-template-select');
    if (!select) return;
    
    if (policyTemplatesCache === null) {
        try {
            policyTemplatesCache = await api('GET', '/_admin/policy-templates');
        } catch (e) {
            policyTemplatesCache = [];
            console.error('Failed to load templates', e);
        }
    }
    
    select.innerHTML = '<option value="">-- Choose a template --</option>';
    policyTemplatesCache.forEach((tpl, index) => {
        const option = document.createElement('option');
        option.value = index;
        option.textContent = tpl.name;
        select.appendChild(option);
    });
}

window.loadPolicyTemplate = () => {
    const select = document.getElementById('policy-template-select');
    if (!select || select.value === "") return;
    
    const index = parseInt(select.value, 10);
    const template = policyTemplatesCache[index];
    if (template) {
        document.getElementById('policy-document').value = template.content;
        document.getElementById('policy-error').style.display = 'none';
        showToast('Template applied. Replace {bucket_name} if applicable!', 'info');
    }
};

async function editPolicy(name) {
    try {
        const policy = await api('GET', `/_admin/policies/${name}`);
        document.getElementById('policy-modal-title').textContent = 'Edit Policy';
        document.getElementById('policy-name').value = name;
        document.getElementById('policy-name').readOnly = true;
        document.getElementById('policy-document').value = JSON.stringify(policy, null, 2);
        document.getElementById('policy-error').style.display = 'none';
        loadPolicyTemplateSelect();
        openModal('policy-modal');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function deletePolicy(name) {
    if (!confirm(`Are you sure you want to delete policy "${name}"?`)) return;
    try {
        await api('DELETE', `/_admin/policies/${name}`);
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
        await api('PUT', `/_admin/policies/${name}`, policy);
        closeModal('policy-modal');
        showToast('Policy saved successfully', 'success');
        fetchPolicies();
    } catch (err) {
        showToast(err.message, 'error');
    }
});

// ==================== Global Settings ====================
window.loadGlobalSettings = async () => {
    try {
        const settings = await api('GET', '/_admin/settings');
        document.getElementById('setting-pagination').value = settings['pagination_size'] || 1000;
        document.getElementById('setting-session-expiry').value = settings['session_expiry_hours'] || 24;
        document.getElementById('setting-cors-allow-origin').value = settings['cors_allow_origin'] || '*';
    } catch (err) {
        showToast('Failed to load global settings: ' + err.message, 'error');
    }
};

window.saveGlobalSettings = async () => {
    const payload = {
        'pagination_size': document.getElementById('setting-pagination').value,
        'session_expiry_hours': document.getElementById('setting-session-expiry').value,
        'cors_allow_origin': document.getElementById('setting-cors-allow-origin').value,
    };
    
    try {
        await api('PUT', '/_admin/settings', payload);
        showToast('Global settings saved successfully', 'success');
    } catch (err) {
        showToast('Failed to save settings: ' + err.message, 'error');
    }
};
