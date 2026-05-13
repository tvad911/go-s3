import { AwsClient } from 'https://esm.sh/aws4fetch@1.0.17';

// State
let s3Client = null;
let currentEndpoint = '';
let currentView = 'buckets';
let currentBucket = null;
let currentPrefix = '';

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

// Initialization
document.addEventListener('DOMContentLoaded', () => {
    // Check saved credentials
    const savedEndpoint = localStorage.getItem('gos3_endpoint');
    const savedAccessKey = localStorage.getItem('gos3_access_key');
    const savedSecretKey = localStorage.getItem('gos3_secret_key');

    if (savedEndpoint && savedAccessKey && savedSecretKey) {
        document.getElementById('endpoint').value = savedEndpoint;
        document.getElementById('access-key').value = savedAccessKey;
        document.getElementById('secret-key').value = savedSecretKey;
        // Auto login
        initClient(savedEndpoint, savedAccessKey, savedSecretKey);
    }
});

// Login
loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const endpoint = document.getElementById('endpoint').value;
    const accessKey = document.getElementById('access-key').value;
    const secretKey = document.getElementById('secret-key').value;

    await initClient(endpoint, accessKey, secretKey);
});

async function initClient(endpoint, accessKey, secretKey) {
    try {
        // Remove trailing slash
        if (endpoint.endsWith('/')) endpoint = endpoint.slice(0, -1);

        s3Client = new AwsClient({
            accessKeyId: accessKey,
            secretAccessKey: secretKey,
            service: 's3',
            region: 'us-east-1'
        });

        currentEndpoint = endpoint;

        // Test connection by listing buckets
        const res = await s3Client.fetch(`${endpoint}/`);
        if (!res.ok) {
            const text = await res.text();
            throw new Error(`Failed to connect: ${res.status} ${text}`);
        }

        // Save credentials
        localStorage.setItem('gos3_endpoint', endpoint);
        localStorage.setItem('gos3_access_key', accessKey);
        localStorage.setItem('gos3_secret_key', secretKey);

        // Switch to dashboard
        loginScreen.classList.remove('active');
        dashboardScreen.classList.add('active');
        
        loadView('buckets');
        showToast('Connected to GoS3', 'success');

    } catch (err) {
        console.error(err);
        loginError.textContent = err.message;
        localStorage.removeItem('gos3_access_key');
        localStorage.removeItem('gos3_secret_key');
    }
}

// Logout
btnLogout.addEventListener('click', () => {
    localStorage.removeItem('gos3_access_key');
    localStorage.removeItem('gos3_secret_key');
    s3Client = null;
    
    dashboardScreen.classList.remove('active');
    loginScreen.classList.add('active');
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
    document.getElementById(`view-${viewName}`).classList.add('active');

    // Update Topbar
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
    } else if (viewName === 'info') {
        breadcrumb.innerHTML = `<span>Server Info</span>`;
        fetchServerInfo();
    }
}

// Buckets Logic
async function fetchBuckets() {
    const grid = document.getElementById('buckets-grid');
    grid.innerHTML = '<div class="loading-spinner"></div>';
    
    try {
        const res = await s3Client.fetch(`${currentEndpoint}/`);
        if (!res.ok) throw new Error('Failed to list buckets');
        
        const text = await res.text();
        const parser = new DOMParser();
        const xml = parser.parseFromString(text, 'text/xml');
        
        const buckets = Array.from(xml.querySelectorAll('Bucket'));
        
        if (buckets.length === 0) {
            grid.innerHTML = '<div style="grid-column: 1/-1; text-align: center; color: var(--text-muted); padding: 2rem;">No buckets found. Create one to get started!</div>';
            return;
        }

        grid.innerHTML = buckets.map(b => {
            const name = b.querySelector('Name').textContent;
            const creationDate = new Date(b.querySelector('CreationDate').textContent).toLocaleString();
            return `
                <div class="bucket-card" onclick="openBucket('${name}')">
                    <div class="bucket-header">
                        <div class="bucket-icon">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z"></path></svg>
                        </div>
                        <div class="bucket-info">
                            <h3>${name}</h3>
                            <p>Created: ${creationDate}</p>
                        </div>
                    </div>
                    <div class="bucket-actions">
                        <button class="btn btn-ghost" onclick="event.stopPropagation(); deleteBucket('${name}')">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                        </button>
                    </div>
                </div>
            `;
        }).join('');

    } catch (err) {
        showToast(err.message, 'error');
        grid.innerHTML = '';
    }
}

// Expose to window for inline handlers
window.openBucket = (name) => {
    currentBucket = name;
    currentPrefix = '';
    
    // Update breadcrumb
    breadcrumb.innerHTML = `
        <span class="clickable" onclick="loadView('buckets')">Buckets</span>
        <span class="separator">/</span>
        <span style="color: var(--accent-primary)">${name}</span>
    `;

    topbarActions.innerHTML = '';

    // Switch view manually without changing nav active state
    views.forEach(v => v.classList.remove('active'));
    document.getElementById('view-objects').classList.add('active');
    
    fetchObjects();
};

document.getElementById('create-bucket-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('new-bucket-name').value;
    try {
        const res = await s3Client.fetch(`${currentEndpoint}/${name}`, { method: 'PUT' });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(`Error: ${res.status} ${text}`);
        }
        showToast(`Bucket ${name} created`, 'success');
        closeModal('create-bucket-modal');
        document.getElementById('new-bucket-name').value = '';
        fetchBuckets();
    } catch (err) {
        showToast(err.message, 'error');
    }
});

window.deleteBucket = async (name) => {
    if (!confirm(`Are you sure you want to delete bucket ${name}?`)) return;
    try {
        const res = await s3Client.fetch(`${currentEndpoint}/${name}`, { method: 'DELETE' });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(`Error: ${res.status} ${text}`);
        }
        showToast(`Bucket ${name} deleted`, 'success');
        fetchBuckets();
    } catch (err) {
        showToast(err.message, 'error');
    }
};

// Objects Logic
async function fetchObjects() {
    const tbody = document.getElementById('objects-table-body');
    tbody.innerHTML = '<tr><td colspan="4" style="text-align: center;"><div class="loading-spinner" style="margin: 1rem auto; width: 24px; height: 24px;"></div></td></tr>';
    
    try {
        const url = new URL(`${currentEndpoint}/${currentBucket}`);
        url.searchParams.append('list-type', '2');
        if (currentPrefix) url.searchParams.append('prefix', currentPrefix);
        
        const res = await s3Client.fetch(url.toString());
        if (!res.ok) throw new Error('Failed to list objects');
        
        const text = await res.text();
        const parser = new DOMParser();
        const xml = parser.parseFromString(text, 'text/xml');
        
        const contents = Array.from(xml.querySelectorAll('Contents'));
        
        if (contents.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" style="text-align: center; color: var(--text-muted);">No objects found in this bucket.</td></tr>';
            return;
        }

        tbody.innerHTML = contents.map(obj => {
            const key = obj.querySelector('Key').textContent;
            const size = parseInt(obj.querySelector('Size').textContent);
            const lastMod = new Date(obj.querySelector('LastModified').textContent).toLocaleString();
            
            return `
                <tr>
                    <td>
                        <div class="file-name">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path><polyline points="13 2 13 9 20 9"></polyline></svg>
                            ${key}
                        </div>
                    </td>
                    <td>${formatBytes(size)}</td>
                    <td>${lastMod}</td>
                    <td>
                        <div class="action-btns">
                            <button class="btn btn-ghost" style="padding: 0.4rem;" onclick="downloadObject('${key}')" title="Download">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>
                            </button>
                            <button class="btn btn-ghost text-danger" style="padding: 0.4rem;" onclick="deleteObject('${key}')" title="Delete">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                            </button>
                        </div>
                    </td>
                </tr>
            `;
        }).join('');

    } catch (err) {
        showToast(err.message, 'error');
        tbody.innerHTML = '';
    }
}

// Upload Handling
const uploadZone = document.getElementById('upload-zone');
const fileInput = document.getElementById('file-input');

uploadZone.addEventListener('click', () => fileInput.click());

uploadZone.addEventListener('dragover', (e) => {
    e.preventDefault();
    uploadZone.classList.add('dragover');
});

uploadZone.addEventListener('dragleave', () => {
    uploadZone.classList.remove('dragover');
});

uploadZone.addEventListener('drop', (e) => {
    e.preventDefault();
    uploadZone.classList.remove('dragover');
    if (e.dataTransfer.files.length) {
        handleUploads(e.dataTransfer.files);
    }
});

fileInput.addEventListener('change', () => {
    if (fileInput.files.length) {
        handleUploads(fileInput.files);
    }
});

async function handleUploads(files) {
    if (!currentBucket) {
        showToast('Please select a bucket first', 'error');
        return;
    }

    for (let i = 0; i < files.length; i++) {
        const file = files[i];
        try {
            showToast(`Uploading ${file.name}...`, 'info');
            const res = await s3Client.fetch(`${currentEndpoint}/${currentBucket}/${currentPrefix}${file.name}`, {
                method: 'PUT',
                body: file
            });
            if (!res.ok) throw new Error(`Failed to upload ${file.name}`);
            showToast(`${file.name} uploaded successfully`, 'success');
        } catch (err) {
            showToast(err.message, 'error');
        }
    }
    fetchObjects();
}

window.downloadObject = async (key) => {
    // Generate presigned URL
    // Actually, aws4fetch doesn't have presign directly out of the box in simple usage,
    // but we can just fetch and blob it, or redirect if it's public.
    // For large files this is bad, but for a simple console it's fine.
    try {
        const url = `${currentEndpoint}/${currentBucket}/${key}`;
        const res = await s3Client.fetch(url);
        if (!res.ok) throw new Error(`Download failed: ${res.status}`);
        
        const blob = await res.blob();
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = key.split('/').pop();
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
    } catch(err) {
        showToast(err.message, 'error');
    }
};

window.deleteObject = async (key) => {
    if (!confirm(`Delete object ${key}?`)) return;
    try {
        const res = await s3Client.fetch(`${currentEndpoint}/${currentBucket}/${key}`, { method: 'DELETE' });
        if (!res.ok) throw new Error(`Delete failed: ${res.status}`);
        showToast(`Object deleted`, 'success');
        fetchObjects();
    } catch(err) {
        showToast(err.message, 'error');
    }
}

// Admin API calls helper
async function doAdminCall(method, path, body = null) {
    let u = currentEndpoint;
    if (u.endsWith('/')) u = u.slice(0, -1);
    
    const opts = { method };
    if (body) {
        opts.body = JSON.stringify(body);
        opts.headers = { 'Content-Type': 'application/json' };
    }
    
    // Use sigv4 to sign the admin request!
    const res = await s3Client.fetch(`${u}/_admin${path}`, opts);
    if (!res.ok) {
        let msg = await res.text();
        throw new Error(msg);
    }
    return await res.json();
}

async function fetchUsers() {
    const tbody = document.getElementById('users-table-body');
    tbody.innerHTML = '<tr><td colspan="2"><div class="loading-spinner"></div></td></tr>';
    
    try {
        const users = await doAdminCall('GET', '/users');
        tbody.innerHTML = users.map(u => `
            <tr>
                <td><strong>${u.username}</strong>${u.is_root ? ' <span class="text-accent" style="font-size:0.8rem; background:rgba(0,240,255,0.1); padding:2px 6px; border-radius:4px; margin-left:8px;">ROOT</span>' : ''}</td>
                <td>
                    ${!u.is_root ? `
                        <button class="btn btn-ghost text-danger" onclick="deleteUser('${u.username}')">Delete</button>
                    ` : ''}
                </td>
            </tr>
        `).join('');
    } catch (err) {
        showToast('Admin API Error: ' + err.message, 'error');
        tbody.innerHTML = '';
    }
}

document.getElementById('create-user-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = document.getElementById('new-username').value;
    try {
        const res = await doAdminCall('POST', '/users', { username });
        alert(`User created!\n\nAccess Key: ${res.accessKey}\nSecret Key: ${res.secretKey}\n\nPlease save these, you won't see them again!`);
        document.getElementById('new-username').value = '';
        fetchUsers();
    } catch(err) {
        showToast(err.message, 'error');
    }
});

window.deleteUser = async (username) => {
    if(!confirm(`Delete user ${username}?`)) return;
    try {
        await doAdminCall('DELETE', `/users/${username}`);
        showToast(`User ${username} deleted`, 'success');
        fetchUsers();
    } catch(err) {
        showToast(err.message, 'error');
    }
}

async function fetchServerInfo() {
    const grid = document.getElementById('info-grid');
    grid.innerHTML = '<div class="loading-spinner"></div>';
    
    try {
        const info = await doAdminCall('GET', '/info');
        grid.innerHTML = `
            <div class="glass-card info-card">
                <span class="info-label">Version</span>
                <span class="info-value" style="color: var(--accent-primary)">${info.version}</span>
            </div>
            <div class="glass-card info-card">
                <span class="info-label">Uptime</span>
                <span class="info-value">${formatDuration(info.uptime_seconds)}</span>
            </div>
            <div class="glass-card info-card">
                <span class="info-label">Total Storage</span>
                <span class="info-value">${formatBytes(info.storage_total_bytes)}</span>
            </div>
            <div class="glass-card info-card">
                <span class="info-label">Free Storage</span>
                <span class="info-value" style="color: var(--success)">${formatBytes(info.storage_free_bytes)}</span>
            </div>
        `;
    } catch (err) {
        showToast('Admin API Error: ' + err.message, 'error');
        grid.innerHTML = '';
    }
}

// Utils
window.openModal = (id) => document.getElementById(id).classList.add('active');
window.closeModal = (id) => document.getElementById(id).classList.remove('active');

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
    
    setTimeout(() => {
        toast.style.animation = 'fadeOut 0.3s ease-out forwards';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

function formatBytes(bytes, decimals = 2) {
    if (!+bytes) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

function formatDuration(seconds) {
    const d = Math.floor(seconds / (3600*24));
    const h = Math.floor(seconds % (3600*24) / 3600);
    const m = Math.floor(seconds % 3600 / 60);
    const s = Math.floor(seconds % 60);
    
    if (d > 0) return `${d}d ${h}h`;
    if (h > 0) return `${h}h ${m}m`;
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
}
